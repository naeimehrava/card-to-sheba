package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/behnam/card-to-sheba/backend/internal/inquiry"
)

const maxRequestBytes = 4 << 10

type InquiryService interface {
	Inquire(ctx context.Context, req inquiry.Request) (*inquiry.Result, error)
}

type Handler struct {
	service           InquiryService
	trustProxyHeaders bool
}

func NewHandler(service InquiryService, trustProxyHeaders bool) *Handler {
	return &Handler{service: service, trustProxyHeaders: trustProxyHeaders}
}

type cardToIBANRequest struct {
	Card string `json:"card"`
}

type cardToIBANResponse struct {
	Data struct {
		MaskedCard      string `json:"maskedCard"`
		IBAN            string `json:"iban"`
		BankName        string `json:"bankName,omitempty"`
		EnglishBankName string `json:"englishBankName,omitempty"`
		Deposit         string `json:"deposit,omitempty"`
		DepositOwners   string `json:"depositOwners,omitempty"`
		DurationMs      int64  `json:"durationMs"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

func (h *Handler) CardToIBAN(w http.ResponseWriter, r *http.Request) {
	var payload cardToIBANRequest
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, r, err)
		return
	}

	result, err := h.service.Inquire(r.Context(), inquiry.Request{
		RequestID: RequestIDFrom(r.Context()),
		Card:      payload.Card,
		ClientIP:  h.clientIP(r),
	})
	if err != nil {

		writeError(w, r, err)
		return
	}

	var body cardToIBANResponse
	body.Data.MaskedCard = result.MaskedCard
	body.Data.IBAN = result.IBAN
	body.Data.BankName = result.BankName
	body.Data.EnglishBankName = result.EnglishBankName
	body.Data.Deposit = result.Deposit
	body.Data.DepositOwners = result.DepositOwners
	body.Data.DurationMs = result.DurationMs
	body.RequestID = result.RequestID

	writeJSON(w, http.StatusOK, body)
}

func Health(checks ...func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		for _, check := range checks {
			if err := check(ctx); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded"})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func (h *Handler) clientIP(r *http.Request) string {
	if h.trustProxyHeaders {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if first, _, found := strings.Cut(fwd, ","); found {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(fwd)
		}
	}
	return ClientIP(r)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return inquiry.NewError(inquiry.KindValidation, "نوع محتوای درخواست باید application/json باشد.", nil)
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return inquiry.NewError(inquiry.KindValidation, "حجم درخواست بیش از حد مجاز است.", err)
		}
		return inquiry.NewError(inquiry.KindValidation, "ساختار درخواست معتبر نیست.", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return inquiry.NewError(inquiry.KindValidation, "بدنه درخواست باید فقط شامل یک شیء JSON باشد.", nil)
	}
	return nil
}
