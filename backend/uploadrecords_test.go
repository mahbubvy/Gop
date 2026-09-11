package backend

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testRecord(dir string, i int) UploadRecord {
	return UploadRecord{UploadRecordSnapshot{filepath.Join(dir, fmt.Sprintf("photo-%d.jpg", i)), 123, 456}, "media-key", 789, "transfer"}
}

func TestUploadRecordsPersistenceMatchingAndIsolation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "records.sqlite")
	s, err := OpenUploadRecordStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	r := testRecord(dir, 0)
	if err := s.SaveConfirmed(ctx, "account-a", []UploadRecord{r}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenUploadRecordStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	check := func(account string, c UploadRecordSnapshot, count int) {
		t.Helper()
		m, err := s.Match(ctx, account, []UploadRecordSnapshot{c})
		if err != nil || len(m) != count {
			t.Fatalf("matches=%v err=%v", m, err)
		}
	}
	check("account-a", r.UploadRecordSnapshot, 1)
	check("account-b", r.UploadRecordSnapshot, 0)
	c := r.UploadRecordSnapshot
	c.Size++
	check("account-a", c, 0)
	c = r.UploadRecordSnapshot
	c.ModifiedNS++
	check("account-a", c, 0)
	c = r.UploadRecordSnapshot
	c.Path = filepath.Join(dir, "other", "photo-0.jpg")
	check("account-a", c, 0)
	r.Size++
	r.MediaKey = "replacement"
	r.Origin = "remote-match"
	if err := s.SaveConfirmed(ctx, "account-a", []UploadRecord{r}); err != nil {
		t.Fatal(err)
	}
	m, err := s.Match(ctx, "account-a", []UploadRecordSnapshot{r.UploadRecordSnapshot})
	if err != nil || m[r.Path].MediaKey != "replacement" {
		t.Fatalf("upsert: %v %v", m, err)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM upload_records").Scan(&count); err != nil || count != 1 {
		t.Fatalf("count %d: %v", count, err)
	}
}

func TestUploadRecordsFailures(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "records.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := testRecord(dir, 0)
	bad := testRecord(dir, 1)
	bad.MediaKey = ""
	if err := s.SaveConfirmed(ctx, "a", []UploadRecord{r, bad}); err == nil {
		t.Fatal("accepted invalid batch")
	}
	m, err := s.Match(ctx, "a", []UploadRecordSnapshot{r.UploadRecordSnapshot})
	if err != nil || len(m) != 0 {
		t.Fatal("partial invalid batch saved")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.SaveConfirmed(cancelled, "a", []UploadRecord{r}); err == nil {
		t.Fatal("accepted cancelled write")
	}
	if m, err := s.Match(cancelled, "a", []UploadRecordSnapshot{r.UploadRecordSnapshot}); err == nil || m != nil {
		t.Fatal("cancelled lookup not failed closed")
	}
	if err := s.SaveConfirmed(ctx, "", []UploadRecord{r}); err == nil {
		t.Fatal("accepted empty account")
	}
	if err := s.SaveConfirmed(ctx, "a", make([]UploadRecord, 257)); err == nil {
		t.Fatal("accepted oversized write")
	}
	if _, err := s.Match(ctx, "a", make([]UploadRecordSnapshot, 257)); err == nil {
		t.Fatal("accepted oversized lookup")
	}
	// A SQL failure in the second row must roll back the first row too.
	if _, err := s.db.Exec("CREATE TRIGGER reject_second BEFORE INSERT ON upload_records WHEN NEW.media_key='reject' BEGIN SELECT RAISE(ABORT,'test failure'); END;"); err != nil {
		t.Fatal(err)
	}
	bad = testRecord(dir, 1)
	bad.MediaKey = "reject"
	if err := s.SaveConfirmed(ctx, "a", []UploadRecord{r, bad}); err == nil {
		t.Fatal("expected SQL failure")
	}
	m, err = s.Match(ctx, "a", []UploadRecordSnapshot{r.UploadRecordSnapshot})
	if err != nil || len(m) != 0 {
		t.Fatal("transaction did not roll back")
	}
	s.Close()
	if m, err := s.Match(ctx, "a", []UploadRecordSnapshot{r.UploadRecordSnapshot}); err == nil || m != nil {
		t.Fatal("closed database yielded matches")
	}
}

func TestUploadRecordsSchemaAndCorruption(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "future.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := OpenUploadRecordStore(ctx, path); err == nil {
		s.Close()
		t.Fatal("accepted future schema")
	}
	path = filepath.Join(dir, "corrupt.sqlite")
	original := []byte("not a sqlite database")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenUploadRecordStore(ctx, path); err == nil {
		s.Close()
		t.Fatal("accepted corruption")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatal("corrupt database was replaced")
	}
}

func TestUploadRecordConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(path, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := CaptureUploadRecordSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ConfirmUploadRecord(before, "key", "transfer", false); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfirmUploadRecord(before, "key", "cancelled", false); err == nil {
		t.Fatal("accepted cancellation origin")
	}
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfirmUploadRecord(before, "key", "transfer", false); err == nil {
		t.Fatal("accepted changed source")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfirmUploadRecord(before, "key", "transfer", false); err == nil {
		t.Fatal("accepted unconfirmed deletion")
	}
	if _, err := ConfirmUploadRecord(before, "key", "transfer", true); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRecordsThousands(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "records.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	const total = 5000
	for start := 0; start < total; start += UploadRecordBatchLimit {
		var batch []UploadRecord
		for i := start; i < min(start+UploadRecordBatchLimit, total); i++ {
			batch = append(batch, testRecord(dir, i))
		}
		if err := s.SaveConfirmed(ctx, "a", batch); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	found := 0
	for start := 0; start < total; start += UploadRecordBatchLimit {
		var batch []UploadRecordSnapshot
		for i := start; i < min(start+UploadRecordBatchLimit, total); i++ {
			batch = append(batch, testRecord(dir, i).UploadRecordSnapshot)
		}
		m, err := s.Match(ctx, "a", batch)
		if err != nil {
			t.Fatal(err)
		}
		found += len(m)
	}
	if found != total {
		t.Fatalf("matched %d/%d", found, total)
	}
	t.Logf("Indexed lookup of %d records: %s (local synthetic test)", total, time.Since(started))
}
