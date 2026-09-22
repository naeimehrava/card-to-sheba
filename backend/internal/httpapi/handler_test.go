package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/naeimehrava/card-to-sheba/backend/internal/inquiry"
)

type stubService struct {
	result  *inquiry.Result
	err     error
	lastReq inquiry.Request
}

func (s *stubService) Inquire(_ context.Context, req inquiry.Request) (*inquiry.Result, error) {
	s.lastReq = req
	return s.result, s.err
}

func newTestRouter(service InquiryService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(service, logger, RouterConfig{AllowedOrigins: []string{"http://localhost:3000"}})
}

func postJSON(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/card-to-iban", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestCardToIBANSuccess(t *testing.T) {
	service := &stubService{result: &inquiry.Result{
		RequestID:       "req-1",
		MaskedCard:      "603799******5670",
		IBAN:            "IR820540102680020817909002",
		BankName:        "بانک نمونه",
		EnglishBankName: "SAMPLE",
		Deposit:         "0101234567001",
		DepositOwners:   "علی رضایی",
		DurationMs:      312,
	}}

	rec := postJSON(t, newTestRouter(service), `{"card":"6037997512345670"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	if rec.Header().Get(RequestIDHeader) == "" {
		t.Error("response is missing the correlation id header")
	}

	var body struct {
		Data struct {
			MaskedCard string `json:"maskedCard"`
			IBAN       string `json:"iban"`
			BankName   string `json:"bankName"`
			DurationMs int64  `json:"durationMs"`
		} `json:"data"`
		RequestID string `json:"requestId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.IBAN != "IR820540102680020817909002" || body.Data.MaskedCard != "603799******5670" {
		t.Errorf("unexpected payload: %+v", body.Data)
	}
	if strings.Contains(rec.Body.String(), "6037997512345670") {
		t.Error("response echoed the full card number back to the client")
	}
	if service.lastReq.RequestID == "" {
		t.Error("handler did not pass a correlation id to the service")
	}
	if service.lastReq.Card != "6037997512345670" {
		t.Errorf("service received card %q", service.lastReq.Card)
	}
}

func TestCardToIBANErrorStatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		kind       inquiry.Kind
		wantStatus int
	}{
		{"validation", inquiry.KindValidation, http.StatusBadRequest},
		{"not found", inquiry.KindNotFound, http.StatusNotFound},
		{"upstream rejected", inquiry.KindUpstreamRejected, http.StatusUnprocessableEntity},
		{"upstream auth", inquiry.KindUpstreamAuth, http.StatusBadGateway},
		{"upstream timeout", inquiry.KindUpstreamTimeout, http.StatusGatewayTimeout},
		{"upstream unavailable", inquiry.KindUpstreamUnavailable, http.StatusServiceUnavailable},
		{"internal", inquiry.KindInternal, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &stubService{err: inquiry.NewError(tt.kind, "پیام ایمن", errors.New("credential=super-secret"))}

			rec := postJSON(t, newTestRouter(service), `{"card":"6037997512345670"}`)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
				RequestID string `json:"requestId"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if body.Error.Code != string(tt.kind) {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.kind)
			}
			if body.RequestID == "" {
				t.Error("error body is missing the request id")
			}
			if strings.Contains(rec.Body.String(), "super-secret") {
				t.Errorf("error body leaked the internal cause: %s", rec.Body.String())
			}
		})
	}
}

func TestCardToIBANValidationErrorCarriesFields(t *testing.T) {

	validationErr := inquiry.ValidateCard("")
	service := &stubService{err: validationErr}

	rec := postJSON(t, newTestRouter(service), `{"card":""}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var body struct {
		Error struct {
			Fields []inquiry.FieldError `json:"fields"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if len(body.Error.Fields) != 1 || body.Error.Fields[0].Field != "card" {
		t.Errorf("fields = %+v, want one entry for card", body.Error.Fields)
	}
}

func TestCardToIBANRejectsMalformedBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"not json", `not-json`},
		{"unknown field", `{"cardNumber":"6037997512345670"}`},
		{"trailing content", `{"card":"6037997512345670"}{"card":"x"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &stubService{result: &inquiry.Result{IBAN: "IR82"}}

			rec := postJSON(t, newTestRouter(service), tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400. body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMethodAndRouteHandling(t *testing.T) {
	handler := newTestRouter(&stubService{result: &inquiry.Result{IBAN: "IR82"}})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/card-to-iban", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", rec.Code)
		}
	})

	t.Run("unknown route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
}

func TestRequestIDIsReusedFromTheCaller(t *testing.T) {
	service := &stubService{result: &inquiry.Result{IBAN: "IR82"}}
	handler := newTestRouter(service)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/card-to-iban", strings.NewReader(`{"card":"6037997512345670"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(RequestIDHeader, "trace-from-gateway")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); got != "trace-from-gateway" {
		t.Errorf("echoed request id = %q, want the incoming one", got)
	}
	if service.lastReq.RequestID != "trace-from-gateway" {
		t.Errorf("service saw request id %q", service.lastReq.RequestID)
	}
}

func TestCORSAllowsOnlyConfiguredOrigins(t *testing.T) {
	handler := newTestRouter(&stubService{result: &inquiry.Result{IBAN: "IR82"}})

	t.Run("allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/card-to-iban", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
			t.Errorf("Access-Control-Allow-Origin = %q", got)
		}
	})

	t.Run("denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/card-to-iban", nil)
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin = %q, want empty for a disallowed origin", got)
		}
	})
}

func TestPanicBecomesUniformInternalError(t *testing.T) {
	handler := newTestRouter(panickingService{})

	rec := postJSON(t, handler, `{"card":"6037997512345670"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("panic detail leaked into the response: %s", rec.Body.String())
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != string(inquiry.KindInternal) {
		t.Errorf("error code = %q, want %q", body.Error.Code, inquiry.KindInternal)
	}
}

type panickingService struct{}

func (panickingService) Inquire(context.Context, inquiry.Request) (*inquiry.Result, error) {
	panic("boom")
}
