package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/naeimehrava/card-to-sheba/backend/internal/inquiry"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "nested", "audit.db"))

	store, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func sampleRecord() inquiry.AuditRecord {
	code := 0
	return inquiry.AuditRecord{
		RequestID:          "req-1",
		CreatedAt:          time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		MaskedCard:         "603799******5670",
		CardFingerprint:    "d0f1a2",
		Status:             inquiry.StatusSuccess,
		UpstreamHTTPStatus: 200,
		UpstreamTrackID:    "track-1",
		UpstreamCode:       &code,
		UpstreamResponse:   `{"data":{"card":"603799******5670"}}`,
		MaskedIBAN:         "IR82******************9002",
		BankName:           "بانک نمونه",
		DurationMs:         412,
		ClientIP:           "203.0.113.9",
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	store := newTestStore(t)

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	var applied int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied migrations = %d, want 1", applied)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	store := newTestStore(t)
	rec := sampleRecord()

	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var (
		maskedCard, status, trackID, response, createdAt string
		upstreamCode, durationMs, httpStatus             int
	)
	err := store.DB().QueryRow(`
		SELECT masked_card, status, upstream_track_id, upstream_response, created_at,
		       upstream_code, duration_ms, upstream_http_status
		FROM inquiries WHERE request_id = ?`, rec.RequestID,
	).Scan(&maskedCard, &status, &trackID, &response, &createdAt, &upstreamCode, &durationMs, &httpStatus)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	if maskedCard != rec.MaskedCard || status != rec.Status || trackID != rec.UpstreamTrackID {
		t.Errorf("round trip mismatch: %q/%q/%q", maskedCard, status, trackID)
	}
	if durationMs != 412 || httpStatus != 200 || upstreamCode != 0 {
		t.Errorf("numeric columns = %d/%d/%d", durationMs, httpStatus, upstreamCode)
	}
	if _, err := time.Parse(time.RFC3339Nano, createdAt); err != nil {
		t.Errorf("created_at is not RFC3339: %q", createdAt)
	}
	if !strings.Contains(response, "603799******5670") {
		t.Errorf("upstream_response = %q", response)
	}
}

func TestSaveFailedAttemptWithoutUpstreamCode(t *testing.T) {
	store := newTestStore(t)

	rec := sampleRecord()
	rec.RequestID = "req-2"
	rec.Status = inquiry.StatusFailed
	rec.ErrorCode = string(inquiry.KindValidation)
	rec.ErrorMessage = "شماره کارت معتبر نیست."
	rec.UpstreamCode = nil
	rec.UpstreamHTTPStatus = 0

	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var code *int
	var errorCode string
	err := store.DB().QueryRow(
		`SELECT upstream_code, error_code FROM inquiries WHERE request_id = ?`, rec.RequestID,
	).Scan(&code, &errorCode)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if code != nil {
		t.Errorf("upstream_code = %v, want NULL when no upstream call happened", *code)
	}
	if errorCode != string(inquiry.KindValidation) {
		t.Errorf("error_code = %q", errorCode)
	}
}

func TestSaveRejectsDuplicateRequestID(t *testing.T) {
	store := newTestStore(t)
	rec := sampleRecord()

	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := store.Save(context.Background(), rec); err == nil {
		t.Error("second Save with the same request id succeeded, want a constraint error")
	}
}

func TestStatusColumnIsConstrained(t *testing.T) {
	store := newTestStore(t)

	_, err := store.DB().Exec(
		`INSERT INTO inquiries (request_id, created_at, masked_card, status) VALUES (?, ?, ?, ?)`,
		"req-3", time.Now().UTC().Format(time.RFC3339Nano), "603799******5670", "bogus",
	)
	if err == nil {
		t.Error("insert with an unknown status succeeded, want the CHECK constraint to reject it")
	}
}
