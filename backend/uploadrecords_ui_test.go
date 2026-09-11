package backend

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadRecordPagesSearchAndForget(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var records []UploadRecord
	for i := 0; i < 105; i++ {
		records = append(records, testRecord(dir, i))
	}
	for _, account := range []string{"a", "b"} {
		if err := s.SaveConfirmed(ctx, account, records); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.page(ctx, "a", "", 0)
	if err != nil || len(first.Rows) != 50 || !first.HasMore {
		t.Fatalf("first %v %v", first, err)
	}
	last, err := s.page(ctx, "a", "", 100)
	if err != nil || len(last.Rows) != 5 || last.HasMore {
		t.Fatalf("last %v %v", last, err)
	}
	search, err := s.page(ctx, "a", "PHOTO-104", 0)
	if err != nil || len(search.Rows) != 1 {
		t.Fatal(search, err)
	}
	if _, err := s.page(ctx, "a", "", -1); err == nil {
		t.Fatal("negative offset accepted")
	}
	if err := s.forget(ctx, "a", []string{records[104].Path}); err != nil {
		t.Fatal(err)
	}
	a, _ := s.page(ctx, "a", "photo-104", 0)
	b, _ := s.page(ctx, "b", "photo-104", 0)
	if len(a.Rows) != 0 || len(b.Rows) != 1 {
		t.Fatal("forget crossed account boundary")
	}
	if err := s.forget(ctx, "a", make([]string, 257)); err == nil {
		t.Fatal("unbounded delete")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.forget(cancelled, "b", []string{records[104].Path}); err == nil {
		t.Fatal("cancelled delete succeeded")
	}
}

func TestUploadRecordPreviewCountsAndCancel(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var paths []string
	for _, name := range []string{"one.jpg", "two.jpg"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("photo"), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	before, _ := CaptureUploadRecordSnapshot(paths[0])
	r, _ := ConfirmUploadRecord(before, "key", "transfer", false)
	if err := s.SaveConfirmed(ctx, "a", []UploadRecord{r}); err != nil {
		t.Fatal(err)
	}
	preview, err := s.preview(ctx, paths, Config{Selected: "a", AlbumAutoMode: true})
	if err != nil || preview.Total != 2 || preview.Matched != 1 || !preview.UsesAlbums {
		t.Fatal(preview, err)
	}
	preview, err = s.preview(ctx, paths, Config{Selected: "b"})
	if err != nil || preview.Matched != 0 || preview.UsesAlbums {
		t.Fatal(preview, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.preview(cancelled, paths, Config{Selected: "a"}); !errors.Is(err, context.Canceled) {
		t.Fatal("preview ignored cancellation", err)
	}
}

func TestUploadRecordCSV(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := testRecord(dir, 1)
	r.Path = filepath.Join(dir, "comma,quote\"newline\n.jpg")
	if err := s.SaveConfirmed(ctx, "a", []UploadRecord{r}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveConfirmed(ctx, "other", []UploadRecord{testRecord(dir, 2)}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := s.exportCSV(ctx, "a", &output); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(output.String())).ReadAll()
	if err != nil || len(rows) != 2 || rows[1][2] != r.Path {
		t.Fatalf("csv %v %v", rows, err)
	}
	if strings.Contains(output.String(), r.MediaKey) || strings.Contains(output.String(), "other") {
		t.Fatal("export leaked key or account")
	}
	for _, value := range []string{"=1+1", "  +SUM(1)", "@formula", "\tdata", "-1"} {
		if !strings.HasPrefix(safeCSVCell(value), "'") {
			t.Fatal("unsafe cell", value)
		}
	}
	if safeCSVCell("") != "" || safeCSVCell("plain") != "plain" {
		t.Fatal("plain value changed")
	}
}
