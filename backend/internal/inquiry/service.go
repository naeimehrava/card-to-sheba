package inquiry

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type Service struct {
	provider       Provider
	audit          AuditRepository
	logger         *slog.Logger
	cardHashSecret string
	now            func() time.Time
}

type Option func(*Service)

func WithCardHashSecret(secret string) Option {
	return func(s *Service) { s.cardHashSecret = secret }
}

func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

func NewService(provider Provider, audit AuditRepository, logger *slog.Logger, opts ...Option) *Service {
	s := &Service{
		provider: provider,
		audit:    audit,
		logger:   logger,
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type Request struct {
	RequestID string
	Card      string
	ClientIP  string
}

type Result struct {
	RequestID       string
	MaskedCard      string
	IBAN            string
	BankName        string
	EnglishBankName string
	Deposit         string
	DepositOwners   string
	DurationMs      int64
}

func (s *Service) Inquire(ctx context.Context, req Request) (*Result, error) {
	start := s.now()
	card := NormalizeCard(req.Card)
	masked := MaskCard(card)

	log := s.logger.With(
		slog.String("request_id", req.RequestID),
		slog.String("masked_card", masked),
	)

	if err := ValidateCard(card); err != nil {
		var svcErr *Error
		errors.As(err, &svcErr)
		log.InfoContext(ctx, "inquiry rejected by validation", slog.String("error_code", string(svcErr.Kind)))
		s.record(ctx, req, masked, "", svcErr, Outcome{}, since(start, s.now))
		return nil, svcErr
	}

	fingerprint := FingerprintCard(card, s.cardHashSecret)
	outcome, err := s.provider.CardToIBAN(ctx, card)
	elapsed := since(start, s.now)

	if err != nil {
		svcErr := asServiceError(err)

		log.WarnContext(ctx, "inquiry failed",
			slog.String("error_code", string(svcErr.Kind)),
			slog.Int("upstream_http_status", outcome.HTTPStatus),
			slog.String("upstream_track_id", outcome.TrackID),
			slog.Int64("duration_ms", elapsed),
			slog.String("cause", svcErr.Error()),
		)
		s.record(ctx, req, masked, fingerprint, svcErr, outcome, elapsed)
		return nil, svcErr
	}

	if outcome.Data == nil || outcome.Data.IBAN == "" {
		svcErr := NewError(KindNotFound, "شبای متناظر با این شماره کارت یافت نشد.", errors.New("provider returned empty IBAN"))
		log.WarnContext(ctx, "inquiry succeeded upstream but returned no IBAN",
			slog.String("upstream_track_id", outcome.TrackID),
			slog.Int64("duration_ms", elapsed),
		)
		s.record(ctx, req, masked, fingerprint, svcErr, outcome, elapsed)
		return nil, svcErr
	}

	log.InfoContext(ctx, "inquiry succeeded",
		slog.String("upstream_track_id", outcome.TrackID),
		slog.String("bank_name", outcome.Data.BankName),
		slog.Int64("duration_ms", elapsed),
	)
	s.record(ctx, req, masked, fingerprint, nil, outcome, elapsed)

	return &Result{
		RequestID:       req.RequestID,
		MaskedCard:      masked,
		IBAN:            outcome.Data.IBAN,
		BankName:        outcome.Data.BankName,
		EnglishBankName: outcome.Data.EnglishBankName,
		Deposit:         outcome.Data.Deposit,
		DepositOwners:   outcome.Data.DepositOwners,
		DurationMs:      elapsed,
	}, nil
}

func (s *Service) record(ctx context.Context, req Request, maskedCard, fingerprint string, svcErr *Error, outcome Outcome, elapsed int64) {
	rec := AuditRecord{
		RequestID:          req.RequestID,
		CreatedAt:          s.now().UTC(),
		MaskedCard:         maskedCard,
		CardFingerprint:    fingerprint,
		Status:             StatusSuccess,
		UpstreamHTTPStatus: outcome.HTTPStatus,
		UpstreamTrackID:    outcome.TrackID,
		UpstreamCode:       outcome.ProviderCode,
		UpstreamResponse:   sanitizeUpstreamJSON(outcome.RawResponse),
		DurationMs:         elapsed,
		ClientIP:           req.ClientIP,
	}
	if svcErr != nil {
		rec.Status = StatusFailed
		rec.ErrorCode = string(svcErr.Kind)
		rec.ErrorMessage = svcErr.Message
	}
	if outcome.Data != nil {
		rec.MaskedIBAN = MaskIBAN(outcome.Data.IBAN)
		rec.BankName = outcome.Data.BankName
	}

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if err := s.audit.Save(writeCtx, rec); err != nil {
		s.logger.ErrorContext(ctx, "failed to persist audit record",
			slog.String("request_id", req.RequestID),
			slog.String("error", err.Error()),
		)
	}
}

func asServiceError(err error) *Error {
	var svcErr *Error
	if errors.As(err, &svcErr) {
		return svcErr
	}
	return NewError(KindInternal, "خطای داخلی سامانه رخ داده است.", err)
}

func since(start time.Time, now func() time.Time) int64 {
	return now().Sub(start).Milliseconds()
}
