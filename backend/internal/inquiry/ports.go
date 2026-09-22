package inquiry

import (
	"context"
	"time"
)

type Provider interface {
	CardToIBAN(ctx context.Context, card string) (Outcome, error)
}

type Outcome struct {
	HTTPStatus   int
	TrackID      string
	ProviderCode *int
	RawResponse  []byte
	Data         *BankDetails
}

type BankDetails struct {
	IBAN               string
	BankName           string
	EnglishBankName    string
	Deposit            string
	DepositOwners      string
	DepositStatus      string
	DepositDescription string
}

type AuditRepository interface {
	Save(ctx context.Context, record AuditRecord) error
}

type AuditRecord struct {
	RequestID          string
	CreatedAt          time.Time
	MaskedCard         string
	CardFingerprint    string
	Status             string
	ErrorCode          string
	ErrorMessage       string
	UpstreamHTTPStatus int
	UpstreamTrackID    string
	UpstreamCode       *int
	UpstreamResponse   string
	MaskedIBAN         string
	BankName           string
	DurationMs         int64
	ClientIP           string
}

const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
)
