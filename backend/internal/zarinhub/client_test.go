package zarinhub

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/behnam/card-to-sheba/backend/internal/inquiry"
)

const testCard = "6037997512345670"

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeZarinHub struct {
	tokenCalls  atomic.Int32
	cardCalls   atomic.Int32
	lastCard    string
	lastAuth    string
	cardStatus  int
	cardBody    string
	tokenStatus int
	tokenBody   string
	delay       time.Duration
}

func (f *fakeZarinHub) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/v5/Authentication/GetToken", func(w http.ResponseWriter, r *http.Request) {
		f.tokenCalls.Add(1)
		status, body := f.tokenStatus, f.tokenBody
		if body == "" {
			status, body = http.StatusOK, successEnvelope(`{"accessToken":"token-1","refreshToken":"refresh-1","tokenType":"Bearer","expiresTn":3600}`)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	})

	mux.HandleFunc("/v5/KYC/CardToIban", func(w http.ResponseWriter, r *http.Request) {
		f.cardCalls.Add(1)
		f.lastAuth = r.Header.Get("Authorization")

		var payload map[string]string
		json.NewDecoder(r.Body).Decode(&payload)
		f.lastCard = payload["card"]

		if f.delay > 0 {
			time.Sleep(f.delay)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.cardStatus)
		io.WriteString(w, f.cardBody)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func successEnvelope(data string) string {
	return `{"data":` + data + `,"meta":{"code":0,"isSuccess":true,"status":"success","trackId":"track-1","errorMessage":null,"errorType":null,"errors":[]}}`
}

func errorEnvelope(code int, errorType, message string) string {
	body, _ := json.Marshal(map[string]any{
		"data": map[string]any{},
		"meta": map[string]any{
			"code": code, "isSuccess": false, "status": "error",
			"trackId": "track-1", "errorType": errorType, "errorMessage": message, "errors": []any{},
		},
	})
	return string(body)
}

func newTestClient(t *testing.T, srv *httptest.Server, timeout time.Duration) *Client {
	t.Helper()
	return New(Config{
		BaseURL:  srv.URL,
		Username: "09120000000",
		Password: "api-password",
		Timeout:  timeout,
	}, testLogger())
}

func TestCardToIBANSuccess(t *testing.T) {
	fake := &fakeZarinHub{
		cardStatus: http.StatusOK,
		cardBody: successEnvelope(`{"bankName":"بانک نمونه","card":"6037997512345670","deposit":"0101234567001",
			"depositOwners":"علی رضایی","englishBankName":"SAMPLE","iban":"IR820540102680020817909002"}`),
	}
	client := newTestClient(t, fake.server(t), 5*time.Second)

	outcome, err := client.CardToIBAN(context.Background(), testCard)
	if err != nil {
		t.Fatalf("CardToIBAN returned error: %v", err)
	}

	if outcome.Data == nil || outcome.Data.IBAN != "IR820540102680020817909002" {
		t.Fatalf("unexpected data: %+v", outcome.Data)
	}
	if outcome.TrackID != "track-1" || outcome.HTTPStatus != http.StatusOK {
		t.Errorf("outcome metadata = %q/%d", outcome.TrackID, outcome.HTTPStatus)
	}
	if len(outcome.RawResponse) == 0 {
		t.Error("raw response was not captured for the audit trail")
	}
	if fake.lastCard != testCard {
		t.Errorf("upstream received card %q, want the documented `card` field", fake.lastCard)
	}
	if fake.lastAuth != "Bearer token-1" {
		t.Errorf("Authorization header = %q", fake.lastAuth)
	}
}

func TestTokenIsFetchedOnceAndReused(t *testing.T) {
	fake := &fakeZarinHub{cardStatus: http.StatusOK, cardBody: successEnvelope(`{"iban":"IR82"}`)}
	client := newTestClient(t, fake.server(t), 5*time.Second)

	for range 3 {
		if _, err := client.CardToIBAN(context.Background(), testCard); err != nil {
			t.Fatalf("CardToIBAN returned error: %v", err)
		}
	}

	if got := fake.tokenCalls.Load(); got != 1 {
		t.Errorf("token requests = %d, want 1 (the token should be cached)", got)
	}
	if got := fake.cardCalls.Load(); got != 3 {
		t.Errorf("card requests = %d, want 3", got)
	}
}

func TestErrorCodeMapping(t *testing.T) {
	tests := []struct {
		name       string
		httpStatus int
		body       string
		wantKind   inquiry.Kind
	}{
		{"bad request", http.StatusBadRequest, errorEnvelope(codeBadRequest, "BadRequest", "پارامترها معتبر نیستند"), inquiry.KindValidation},
		{"not found", http.StatusOK, errorEnvelope(codeNotFound, "NotFound", "یافت نشد"), inquiry.KindNotFound},
		{"list empty", http.StatusOK, errorEnvelope(codeListEmpty, "ListEmpty", "لیست خالی است"), inquiry.KindNotFound},
		{"unauthorized", http.StatusUnauthorized, errorEnvelope(codeUnauthorized, "UnAuthorized", "خطای احراز هویت"), inquiry.KindUpstreamAuth},
		{"inaccessible", http.StatusForbidden, errorEnvelope(codeInaccessible, "Inaccessible", "عدم دسترسی"), inquiry.KindUpstreamAuth},
		{"unavailable", http.StatusServiceUnavailable, errorEnvelope(codeUnavailable, "Unavailable", "در دسترس نیست"), inquiry.KindUpstreamUnavailable},
		{"server error", http.StatusInternalServerError, errorEnvelope(codeServerError, "ServerError", "خطای سرور"), inquiry.KindUpstreamUnavailable},
		{"upstream timeout", http.StatusOK, errorEnvelope(codeRequestTimout, "RequestTimeOut", "منقضی شد"), inquiry.KindUpstreamTimeout},
		{"logic error", http.StatusBadRequest, errorEnvelope(codeLogicError, "LogicError", "موجودی ناکافی"), inquiry.KindUpstreamRejected},
		{"provider error", http.StatusBadGateway, errorEnvelope(codeProviderError, "ProviderError", "خطای تامین کننده"), inquiry.KindUpstreamRejected},
		{"unparsable body", http.StatusBadGateway, `<html>gateway error</html>`, inquiry.KindUpstreamUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeZarinHub{cardStatus: tt.httpStatus, cardBody: tt.body}
			client := newTestClient(t, fake.server(t), 5*time.Second)

			outcome, err := client.CardToIBAN(context.Background(), testCard)

			var svcErr *inquiry.Error
			if !errors.As(err, &svcErr) {
				t.Fatalf("error = %v (%T), want *inquiry.Error", err, err)
			}
			if svcErr.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", svcErr.Kind, tt.wantKind)
			}
			if outcome.HTTPStatus != tt.httpStatus {
				t.Errorf("outcome.HTTPStatus = %d, want %d (needed for the audit trail)", outcome.HTTPStatus, tt.httpStatus)
			}
		})
	}
}

func TestUpstreamMessageIsNotExposedToTheUser(t *testing.T) {
	fake := &fakeZarinHub{
		cardStatus: http.StatusBadRequest,
		cardBody:   errorEnvelope(codeLogicError, "LogicError", "موجودی حساب شما کافی نیست"),
	}
	client := newTestClient(t, fake.server(t), 5*time.Second)

	_, err := client.CardToIBAN(context.Background(), testCard)

	var svcErr *inquiry.Error
	if !errors.As(err, &svcErr) {
		t.Fatalf("error = %v, want *inquiry.Error", err)
	}
	if strings.Contains(svcErr.Message, "موجودی") {
		t.Errorf("user-facing message disclosed the account state: %q", svcErr.Message)
	}
	if !strings.Contains(svcErr.Error(), "موجودی") {
		t.Error("the private cause lost the upstream detail needed for debugging")
	}
}

func TestExpiredTokenTriggersASingleRefresh(t *testing.T) {
	fake := &fakeZarinHub{}
	mux := http.NewServeMux()

	var tokenCalls, cardCalls atomic.Int32
	mux.HandleFunc("/v5/Authentication/GetToken", func(w http.ResponseWriter, _ *http.Request) {
		n := tokenCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, successEnvelope(`{"accessToken":"token-`+string(rune('0'+n))+`","refreshToken":"refresh-1","expiresTn":3600}`))
	})
	mux.HandleFunc("/v5/KYC/CardToIban", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cardCalls.Add(1) == 1 {

			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, errorEnvelope(codeUnauthorized, "UnAuthorized", "توکن منقضی است"))
			return
		}
		fake.lastAuth = r.Header.Get("Authorization")
		io.WriteString(w, successEnvelope(`{"iban":"IR820540102680020817909002"}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := newTestClient(t, srv, 5*time.Second)

	outcome, err := client.CardToIBAN(context.Background(), testCard)
	if err != nil {
		t.Fatalf("CardToIBAN did not recover from an expired token: %v", err)
	}
	if outcome.Data.IBAN != "IR820540102680020817909002" {
		t.Errorf("IBAN = %q", outcome.Data.IBAN)
	}
	if got := cardCalls.Load(); got != 2 {
		t.Errorf("card requests = %d, want 2 (one retry only)", got)
	}
	if got := tokenCalls.Load(); got != 2 {
		t.Errorf("token requests = %d, want 2", got)
	}
}

func TestPersistentAuthFailureDoesNotLoop(t *testing.T) {
	var cardCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v5/Authentication/GetToken", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, successEnvelope(`{"accessToken":"token-1","expiresTn":3600}`))
	})
	mux.HandleFunc("/v5/KYC/CardToIban", func(w http.ResponseWriter, _ *http.Request) {
		cardCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, errorEnvelope(codeUnauthorized, "UnAuthorized", "توکن نامعتبر"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := newTestClient(t, srv, 5*time.Second)

	_, err := client.CardToIBAN(context.Background(), testCard)

	var svcErr *inquiry.Error
	if !errors.As(err, &svcErr) || svcErr.Kind != inquiry.KindUpstreamAuth {
		t.Fatalf("error = %v, want %s", err, inquiry.KindUpstreamAuth)
	}
	if got := cardCalls.Load(); got != 2 {
		t.Errorf("card requests = %d, want 2 (initial attempt plus one retry)", got)
	}
}

func TestTimeoutIsItsOwnErrorKind(t *testing.T) {
	fake := &fakeZarinHub{
		cardStatus: http.StatusOK,
		cardBody:   successEnvelope(`{"iban":"IR82"}`),
		delay:      300 * time.Millisecond,
	}
	client := newTestClient(t, fake.server(t), 50*time.Millisecond)

	_, err := client.CardToIBAN(context.Background(), testCard)

	var svcErr *inquiry.Error
	if !errors.As(err, &svcErr) {
		t.Fatalf("error = %v, want *inquiry.Error", err)
	}
	if svcErr.Kind != inquiry.KindUpstreamTimeout {
		t.Errorf("Kind = %q, want %q", svcErr.Kind, inquiry.KindUpstreamTimeout)
	}
}

func TestUnreachableUpstreamIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	url := srv.URL
	srv.Close()

	client := New(Config{BaseURL: url, StaticToken: "token-1", Timeout: 2 * time.Second}, testLogger())

	_, err := client.CardToIBAN(context.Background(), testCard)

	var svcErr *inquiry.Error
	if !errors.As(err, &svcErr) || svcErr.Kind != inquiry.KindUpstreamUnavailable {
		t.Fatalf("error = %v, want %s", err, inquiry.KindUpstreamUnavailable)
	}
}

func TestStaticTokenSkipsTheTokenEndpoint(t *testing.T) {
	fake := &fakeZarinHub{cardStatus: http.StatusOK, cardBody: successEnvelope(`{"iban":"IR82"}`)}
	srv := fake.server(t)
	client := New(Config{BaseURL: srv.URL, StaticToken: "preissued-token", Timeout: 5 * time.Second}, testLogger())

	if _, err := client.CardToIBAN(context.Background(), testCard); err != nil {
		t.Fatalf("CardToIBAN returned error: %v", err)
	}
	if got := fake.tokenCalls.Load(); got != 0 {
		t.Errorf("token requests = %d, want 0 when a static token is configured", got)
	}
	if fake.lastAuth != "Bearer preissued-token" {
		t.Errorf("Authorization header = %q", fake.lastAuth)
	}
}
