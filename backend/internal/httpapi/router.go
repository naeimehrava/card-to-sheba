package httpapi

import (
	"context"
	"log/slog"
	"net/http"
)

type RouterConfig struct {
	AllowedOrigins    []string
	TrustProxyHeaders bool
}

func NewRouter(service InquiryService, logger *slog.Logger, cfg RouterConfig, healthChecks ...func(context.Context) error) http.Handler {
	handler := NewHandler(service, cfg.TrustProxyHeaders)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/card-to-iban", handler.CardToIBAN)
	mux.HandleFunc("GET /api/v1/health", Health(healthChecks...))

	mux.HandleFunc("/api/v1/card-to-iban", methodNotAllowed(http.MethodPost))
	mux.HandleFunc("/api/v1/health", methodNotAllowed(http.MethodGet))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, notFoundBody(r))
	})

	return Chain(mux,
		RequestID,
		Recover(logger),
		AccessLog(logger),
		CORS(cfg.AllowedOrigins),
	)
}

func methodNotAllowed(allowed string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body errorBody
		body.Error.Code = "METHOD_NOT_ALLOWED"
		body.Error.Message = "این متد برای مسیر درخواستی پشتیبانی نمی‌شود."
		body.RequestID = RequestIDFrom(r.Context())

		w.Header().Set("Allow", allowed)
		writeJSON(w, http.StatusMethodNotAllowed, body)
	}
}

func notFoundBody(r *http.Request) errorBody {
	var body errorBody
	body.Error.Code = "ROUTE_NOT_FOUND"
	body.Error.Message = "مسیر درخواستی یافت نشد."
	body.RequestID = RequestIDFrom(r.Context())
	return body
}
