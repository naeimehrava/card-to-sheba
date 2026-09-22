package inquiry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	validCard   = "6037997512345670"
	invalidCard = "6037997512345678"
)

type fakeProvider struct {
	outcome  Outcome
	err      error
	calls    int
	lastCard string
}

func (f *fakeProvider) CardToIBAN(_ context.Context, card string) (Outcome, error) {
	f.calls++
	f.lastCard = card
	return f.outcome, f.err
}

type fakeAudit struct {
	mu      sync.Mutex
	records []AuditRecord
	err     error
}

func (f *fakeAudit) Save(_ context.Context, record AuditRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, record)
	return f.err
}

func (f *fakeAudit) only(t *testing.T) AuditRecord {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.records) != 1 {
		t.Fatalf("audit records = %d, want exactly 1", len(f.records))
	}
	return f.records[0]
}

func newTestService(t *testing.T, provider Provider, audit AuditRepository, opts ...Option) *Service {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(provider, audit, logger, opts...)
}

func successOutcome() Outcome {
	code := 0
	return Outcome{
		HTTPStatus:   200,
		TrackID:      "9b126e9a-184c-4cb0-a47e-62b9a3b7176f",
		ProviderCode: &code,
		RawResponse:  []byte(`{"data":{"card":"6037997512345670","iban":"IR820540102680020817909002","bankName":"بانک نمونه"},"meta":{"code":0,"isSuccess":true,"trackId":"9b126e9a-184c-4cb0-a47e-62b9a3b7176f"}}`),
		Data: &BankDetails{
			IBAN:            "IR820540102680020817909002",
			BankName:        "بانک نمونه",
			EnglishBankName: "SAMPLE",
			Deposit:         "0101234567001",
			DepositOwners:   "علی رضایی",
		},
	}
}

func TestInquireSuccess(t *testing.T) {
	provider := &fakeProvider{outcome: successOutcome()}
	audit := &fakeAudit{}
	svc := newTestService(t, provider, audit, WithCardHashSecret("test-secret"))

	result, err := svc.Inquire(context.Background(), Request{
		RequestID: "req-1",
		Card:      " ۶۰۳۷-۹۹۷۵ ۱۲۳۴ ۵۶۷۰ ",
		ClientIP:  "203.0.113.9",
	})
	if err != nil {
		t.Fatalf("Inquire returned error: %v", err)
	}

	if provider.lastCard != validCard {
		t.Errorf("provider received %q, want the normalized card %q", provider.lastCard, validCard)
	}
	if result.IBAN != "IR820540102680020817909002" {
		t.Errorf("IBAN = %q", result.IBAN)
	}
	if result.MaskedCard != "603799******5670" {
		t.Errorf("MaskedCard = %q, want the masked form", result.MaskedCard)
	}

	rec := audit.only(t)
	if rec.Status != StatusSuccess {
		t.Errorf("audit status = %q, want %q", rec.Status, StatusSuccess)
	}
	if rec.RequestID != "req-1" || rec.ClientIP != "203.0.113.9" {
		t.Errorf("audit correlation fields = %q/%q", rec.RequestID, rec.ClientIP)
	}
	if rec.UpstreamTrackID != "9b126e9a-184c-4cb0-a47e-62b9a3b7176f" {
		t.Errorf("audit lost the upstream track id: %q", rec.UpstreamTrackID)
	}
	if rec.CardFingerprint == "" {
		t.Error("audit fingerprint is empty despite a configured secret")
	}
	if rec.UpstreamResponse == "" {
		t.Error("audit did not keep the upstream response")
	}
}

func TestInquireNeverPersistsTheFullCard(t *testing.T) {
	provider := &fakeProvider{outcome: successOutcome()}
	audit := &fakeAudit{}
	svc := newTestService(t, provider, audit, WithCardHashSecret("test-secret"))

	if _, err := svc.Inquire(context.Background(), Request{RequestID: "req-1", Card: validCard}); err != nil {
		t.Fatalf("Inquire returned error: %v", err)
	}

	rec := audit.only(t)
	for field, value := range map[string]string{
		"masked_card":       rec.MaskedCard,
		"upstream_response": rec.UpstreamResponse,
		"card_fingerprint":  rec.CardFingerprint,
		"masked_iban":       rec.MaskedIBAN,
	} {
		if strings.Contains(value, validCard) {
			t.Errorf("audit field %s contains the full card number: %q", field, value)
		}
	}
	if !strings.Contains(rec.UpstreamResponse, "603799******5670") {
		t.Errorf("upstream response was not masked in place: %q", rec.UpstreamResponse)
	}
}

func TestInquireRejectsInvalidCardWithoutCallingProvider(t *testing.T) {
	provider := &fakeProvider{outcome: successOutcome()}
	audit := &fakeAudit{}
	svc := newTestService(t, provider, audit)

	_, err := svc.Inquire(context.Background(), Request{RequestID: "req-2", Card: invalidCard})

	var svcErr *Error
	if !errors.As(err, &svcErr) || svcErr.Kind != KindValidation {
		t.Fatalf("error = %v, want a %s", err, KindValidation)
	}
	if provider.calls != 0 {
		t.Errorf("provider was called %d times for invalid input, want 0", provider.calls)
	}

	rec := audit.only(t)
	if rec.Status != StatusFailed || rec.ErrorCode != string(KindValidation) {
		t.Errorf("audit = %q/%q, want a failed validation record", rec.Status, rec.ErrorCode)
	}
	if rec.UpstreamHTTPStatus != 0 {
		t.Errorf("audit recorded an upstream status for a call that never happened: %d", rec.UpstreamHTTPStatus)
	}
}

func TestInquirePropagatesProviderErrorKinds(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
	}{
		{"timeout", KindUpstreamTimeout},
		{"unavailable", KindUpstreamUnavailable},
		{"auth", KindUpstreamAuth},
		{"rejected", KindUpstreamRejected},
		{"not found", KindNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := 7
			provider := &fakeProvider{
				outcome: Outcome{HTTPStatus: 503, TrackID: "track-1", ProviderCode: &code, RawResponse: []byte(`{"meta":{"code":7}}`)},
				err:     NewError(tt.kind, "پیام ایمن برای کاربر", errors.New("secret upstream detail")),
			}
			audit := &fakeAudit{}
			svc := newTestService(t, provider, audit)

			_, err := svc.Inquire(context.Background(), Request{RequestID: "req-3", Card: validCard})

			var svcErr *Error
			if !errors.As(err, &svcErr) {
				t.Fatalf("error = %T, want *Error", err)
			}
			if svcErr.Kind != tt.kind {
				t.Errorf("Kind = %q, want %q", svcErr.Kind, tt.kind)
			}
			if strings.Contains(svcErr.Message, "secret upstream detail") {
				t.Errorf("user-facing message leaked the internal cause: %q", svcErr.Message)
			}

			rec := audit.only(t)
			if rec.Status != StatusFailed || rec.ErrorCode != string(tt.kind) {
				t.Errorf("audit = %q/%q, want a failed %s record", rec.Status, rec.ErrorCode, tt.kind)
			}
			if rec.UpstreamHTTPStatus != 503 || rec.UpstreamTrackID != "track-1" {
				t.Errorf("audit lost upstream metadata: status=%d track=%q", rec.UpstreamHTTPStatus, rec.UpstreamTrackID)
			}
		})
	}
}

func TestInquireTreatsEmptyIBANAsNotFound(t *testing.T) {
	outcome := successOutcome()
	outcome.Data.IBAN = ""
	provider := &fakeProvider{outcome: outcome}
	audit := &fakeAudit{}
	svc := newTestService(t, provider, audit)

	_, err := svc.Inquire(context.Background(), Request{RequestID: "req-4", Card: validCard})

	var svcErr *Error
	if !errors.As(err, &svcErr) || svcErr.Kind != KindNotFound {
		t.Fatalf("error = %v, want a %s", err, KindNotFound)
	}
	if rec := audit.only(t); rec.Status != StatusFailed {
		t.Errorf("audit status = %q, want %q", rec.Status, StatusFailed)
	}
}

func TestInquireWrapsUnknownProviderErrorsAsInternal(t *testing.T) {
	provider := &fakeProvider{err: errors.New("some unexpected failure")}
	svc := newTestService(t, provider, &fakeAudit{})

	_, err := svc.Inquire(context.Background(), Request{RequestID: "req-5", Card: validCard})

	var svcErr *Error
	if !errors.As(err, &svcErr) || svcErr.Kind != KindInternal {
		t.Fatalf("error = %v, want a %s", err, KindInternal)
	}
	if strings.Contains(svcErr.Message, "some unexpected failure") {
		t.Errorf("internal detail leaked into the user message: %q", svcErr.Message)
	}
}

func TestInquireSucceedsWhenAuditWriteFails(t *testing.T) {
	provider := &fakeProvider{outcome: successOutcome()}
	audit := &fakeAudit{err: errors.New("disk full")}
	svc := newTestService(t, provider, audit)

	result, err := svc.Inquire(context.Background(), Request{RequestID: "req-6", Card: validCard})
	if err != nil {
		t.Fatalf("Inquire failed because of an audit error: %v", err)
	}
	if result.IBAN == "" {
		t.Error("result is empty despite a successful upstream call")
	}
}

func TestInquireAuditsEvenWhenRequestContextIsCancelled(t *testing.T) {
	provider := &fakeProvider{outcome: successOutcome()}
	audit := &cancelAwareAudit{}
	svc := newTestService(t, provider, audit)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := svc.Inquire(ctx, Request{RequestID: "req-7", Card: validCard}); err != nil {
		t.Fatalf("Inquire returned error: %v", err)
	}
	if audit.sawCancelledContext {
		t.Error("audit write inherited the cancelled request context")
	}
	if audit.saves != 1 {
		t.Errorf("audit saves = %d, want 1", audit.saves)
	}
}

type cancelAwareAudit struct {
	saves               int
	sawCancelledContext bool
}

func (c *cancelAwareAudit) Save(ctx context.Context, _ AuditRecord) error {
	c.saves++
	if ctx.Err() != nil {
		c.sawCancelledContext = true
	}
	return nil
}

func TestInquireRecordsDuration(t *testing.T) {
	provider := &fakeProvider{outcome: successOutcome()}
	audit := &fakeAudit{}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	clock := func() time.Time {

		offsets := []time.Duration{0, 250 * time.Millisecond, 250 * time.Millisecond}
		idx := min(calls, len(offsets)-1)
		calls++
		return base.Add(offsets[idx])
	}

	svc := newTestService(t, provider, audit, WithClock(clock))

	result, err := svc.Inquire(context.Background(), Request{RequestID: "req-8", Card: validCard})
	if err != nil {
		t.Fatalf("Inquire returned error: %v", err)
	}
	if result.DurationMs != 250 {
		t.Errorf("DurationMs = %d, want 250", result.DurationMs)
	}
	if rec := audit.only(t); rec.DurationMs != 250 {
		t.Errorf("audit DurationMs = %d, want 250", rec.DurationMs)
	}
}
