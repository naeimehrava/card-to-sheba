package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/behnam/card-to-sheba/backend/internal/inquiry"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	if err := ensureParentDir(dsn); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	store := &Store{db: db}
	if err := store.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Migrate(ctx context.Context) error {
	const createTracking = `CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`
	if _, err := s.db.ExecContext(ctx, createTracking); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)

	for _, entry := range entries {
		name := filepath.Base(entry)

		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM schema_migrations WHERE name = ?`, name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists > 0 {
			continue
		}

		body, err := migrationFS.ReadFile(entry)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`,
			name, time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) Save(ctx context.Context, rec inquiry.AuditRecord) error {
	const query = `INSERT INTO inquiries (
		request_id, created_at, masked_card, card_fingerprint, status,
		error_code, error_message, upstream_http_status, upstream_track_id,
		upstream_code, upstream_response, masked_iban, bank_name,
		duration_ms, client_ip
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var upstreamCode any
	if rec.UpstreamCode != nil {
		upstreamCode = *rec.UpstreamCode
	}

	_, err := s.db.ExecContext(ctx, query,
		rec.RequestID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.MaskedCard,
		rec.CardFingerprint,
		rec.Status,
		rec.ErrorCode,
		rec.ErrorMessage,
		rec.UpstreamHTTPStatus,
		rec.UpstreamTrackID,
		upstreamCode,
		rec.UpstreamResponse,
		rec.MaskedIBAN,
		rec.BankName,
		rec.DurationMs,
		rec.ClientIP,
	)
	if err != nil {
		return fmt.Errorf("insert inquiry audit record: %w", err)
	}
	return nil
}

func ensureParentDir(dsn string) error {
	path := dsn
	if after, ok := strings.CutPrefix(path, "file:"); ok {
		path = after
	}
	if q := strings.IndexByte(path, '?'); q >= 0 {
		path = path[:q]
	}
	if path == "" || path == ":memory:" || strings.Contains(dsn, "mode=memory") {
		return nil
	}
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}

	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create database directory %q: %w", dir, err)
	}
	return nil
}
