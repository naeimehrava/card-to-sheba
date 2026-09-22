package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/naeimehrava/card-to-sheba/backend/internal/inquiry"
)

type errorBody struct {
	Error struct {
		Code    string               `json:"code"`
		Message string               `json:"message"`
		Fields  []inquiry.FieldError `json:"fields,omitempty"`
	} `json:"error"`
	RequestID string `json:"requestId,omitempty"`
}

func statusForKind(kind inquiry.Kind) int {
	switch kind {
	case inquiry.KindValidation:
		return http.StatusBadRequest
	case inquiry.KindNotFound:
		return http.StatusNotFound
	case inquiry.KindUpstreamRejected:
		return http.StatusUnprocessableEntity
	case inquiry.KindUpstreamAuth:

		return http.StatusBadGateway
	case inquiry.KindUpstreamTimeout:
		return http.StatusGatewayTimeout
	case inquiry.KindUpstreamUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	svcErr := &inquiry.Error{}
	if !errors.As(err, &svcErr) {
		svcErr = inquiry.NewError(inquiry.KindInternal, "خطای داخلی سامانه رخ داده است.", err)
	}

	var body errorBody
	body.Error.Code = string(svcErr.Kind)
	body.Error.Message = svcErr.Message
	body.Error.Fields = svcErr.Fields
	body.RequestID = RequestIDFrom(r.Context())

	writeJSON(w, statusForKind(svcErr.Kind), body)
}
