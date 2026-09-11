package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// UploadRecordBatchLimit bounds each transaction and lookup. Callers stream batches.
const UploadRecordBatchLimit = 256

var errRecordedMediaKeyMissing = errors.New("recorded file has no saved media key; bypass local records to retry normally")

type UploadRecordSnapshot struct {
	Path       string
	Size       int64
	ModifiedNS int64
}

type UploadRecord struct {
	UploadRecordSnapshot
	MediaKey     string
	RecordedAtNS int64
	// Origin is "transfer" or "remote-match", never a generic skip.
	Origin string
}

type UploadRecordStore struct{ db *sql.DB }

func DefaultUploadRecordPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gotohp", "upload-records.sqlite"), nil
}

func CaptureUploadRecordSnapshot(path string) (UploadRecordSnapshot, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return UploadRecordSnapshot{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return UploadRecordSnapshot{}, err
	}
	if !info.Mode().IsRegular() {
		return UploadRecordSnapshot{}, errors.New("record source is not a regular file")
	}
	return UploadRecordSnapshot{filepath.Clean(abs), info.Size(), info.ModTime().UnixNano()}, nil
}

// ConfirmUploadRecord must only be called after confirmed media success. A missing
// source is accepted only when the caller confirms its own successful cleanup.
func ConfirmUploadRecord(before UploadRecordSnapshot, mediaKey, origin string, deletedAfterSuccess bool) (UploadRecord, error) {
	after, err := CaptureUploadRecordSnapshot(before.Path)
	if err != nil && !(deletedAfterSuccess && errors.Is(err, os.ErrNotExist)) {
		return UploadRecord{}, err
	}
	if err == nil && before != after {
		return UploadRecord{}, errors.New("source changed during upload")
	}
	r := UploadRecord{before, mediaKey, time.Now().UTC().UnixNano(), origin}
	return r, validateUploadRecord(r)
}

func validateUploadRecord(r UploadRecord) error {
	if !filepath.IsAbs(r.Path) || filepath.Clean(r.Path) != r.Path || r.Size < 0 ||
		strings.TrimSpace(r.MediaKey) == "" || r.RecordedAtNS <= 0 ||
		(r.Origin != "transfer" && r.Origin != "remote-match") {
		return errors.New("invalid confirmed upload record")
	}
	return nil
}

// OpenUploadRecordStore never deletes or replaces an unreadable database.
// Upload batches open the store only when recording or local skipping is enabled.
func OpenUploadRecordStore(ctx context.Context, path string) (*UploadRecordStore, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("database path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &UploadRecordStore{db}
	if err = s.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *UploadRecordStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout = 3000"); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 1 {
		return fmt.Errorf("unsupported upload records schema %d", version)
	}
	if version == 0 {
		_, err = tx.ExecContext(ctx, `CREATE TABLE upload_records (
		 account TEXT NOT NULL, path TEXT NOT NULL, size INTEGER NOT NULL CHECK(size >= 0),
		 modified_ns INTEGER NOT NULL, media_key TEXT NOT NULL CHECK(length(media_key)>0),
		 recorded_at_ns INTEGER NOT NULL, origin TEXT NOT NULL CHECK(origin IN ('transfer','remote-match')),
		 PRIMARY KEY(account, path)
		) WITHOUT ROWID; PRAGMA user_version = 1;`)
		if err != nil {
			return err
		}
	}
	// Validate the expected columns even when the version already matches.
	rows, err := tx.QueryContext(ctx, "SELECT account,path,size,modified_ns,media_key,recorded_at_ns,origin FROM upload_records LIMIT 0")
	if err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *UploadRecordStore) Close() error { return s.db.Close() }

// SaveConfirmed commits one bounded batch atomically. There is no hidden memory
// buffer: on return, success means committed; failure means callers must warn.
func (s *UploadRecordStore) SaveConfirmed(ctx context.Context, account string, records []UploadRecord) error {
	if strings.TrimSpace(account) == "" || len(records) > UploadRecordBatchLimit {
		return errors.New("invalid account or oversized record batch")
	}
	for _, r := range records {
		if err := validateUploadRecord(r); err != nil {
			return err
		}
	}
	if len(records) == 0 {
		return ctx.Err()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO upload_records VALUES(?,?,?,?,?,?,?)
	 ON CONFLICT(account,path) DO UPDATE SET size=excluded.size, modified_ns=excluded.modified_ns,
	 media_key=excluded.media_key, recorded_at_ns=excluded.recorded_at_ns, origin=excluded.origin`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, account, r.Path, r.Size, r.ModifiedNS, r.MediaKey, r.RecordedAtNS, r.Origin); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Match returns only exact metadata matches; any database error returns no matches.
func (s *UploadRecordStore) Match(ctx context.Context, account string, candidates []UploadRecordSnapshot) (map[string]UploadRecord, error) {
	if strings.TrimSpace(account) == "" || len(candidates) > UploadRecordBatchLimit {
		return nil, errors.New("invalid account or oversized lookup batch")
	}
	matches := make(map[string]UploadRecord)
	if len(candidates) == 0 {
		return matches, ctx.Err()
	}
	wanted := make(map[string]UploadRecordSnapshot, len(candidates))
	args := []any{account}
	for _, c := range candidates {
		if !filepath.IsAbs(c.Path) || filepath.Clean(c.Path) != c.Path || c.Size < 0 {
			return nil, errors.New("invalid candidate metadata")
		}
		wanted[c.Path] = c
		args = append(args, c.Path)
	}
	query := "SELECT path,size,modified_ns,media_key,recorded_at_ns,origin FROM upload_records WHERE account=? AND path IN (" + strings.TrimSuffix(strings.Repeat("?,", len(candidates)), ",") + ")"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r UploadRecord
		if err := rows.Scan(&r.Path, &r.Size, &r.ModifiedNS, &r.MediaKey, &r.RecordedAtNS, &r.Origin); err != nil {
			return nil, err
		}
		if r.UploadRecordSnapshot == wanted[r.Path] && strings.TrimSpace(r.MediaKey) == "" {
			return nil, errRecordedMediaKeyMissing
		}
		if err := validateUploadRecord(r); err != nil {
			return nil, err
		}
		if r.UploadRecordSnapshot == wanted[r.Path] {
			matches[r.Path] = r
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}
