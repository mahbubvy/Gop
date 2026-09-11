package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordBatchReuseAndBypasses(t *testing.T) {
	for _, mode := range []string{"reuse", "force", "delete", "bypass", "disabled", "account", "live"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			path := filepath.Join(dir, "photo.jpg")
			if err := os.WriteFile(path, []byte("photo"), 0600); err != nil {
				t.Fatal(err)
			}
			s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "db.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			before, err := CaptureUploadRecordSnapshot(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := ConfirmUploadRecord(before, "saved", "transfer", false)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SaveConfirmed(ctx, "a", []UploadRecord{r}); err != nil {
				t.Fatal(err)
			}
			b := &recordBatch{store: s, config: Config{Selected: "a", SkipRecordedUploads: true}}
			item := UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}}
			switch mode {
			case "force":
				b.config.ForceUpload = true
			case "delete":
				b.config.DeleteFromHost = true
			case "bypass":
				b.bypass = true
			case "disabled":
				b.config.SkipRecordedUploads = false
			case "account":
				b.config.Selected = "b"
			case "live":
				item = UploadWorkItem{Kind: UploadWorkLivePhoto}
			}
			calls := 0
			var delta int64
			key, skip, local, err := b.run(ctx, item, func(event string, data any) {
				if event == "uploadTotalBytesDelta" {
					delta += data.(int64)
				}
			}, func(ctx context.Context, cb ProgressCallback) (string, bool, error) {
				calls++
				if uploadConfig(ctx).Selected != b.config.Selected {
					t.Fatal("lost snapshot")
				}
				return "normal", false, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "reuse" {
				if calls != 0 || key != "saved" || !skip || !local || delta != -5 {
					t.Fatalf("bad reuse %s %v %v %d %d", key, skip, local, calls, delta)
				}
			} else if calls != 1 || local || skip || delta != 0 {
				t.Fatal("bypass failed")
			}
		})
	}
}

func TestRecordBatchSuccessFailureCancellation(t *testing.T) {
	for _, mode := range []string{"success", "remote", "failure", "cancel-after-success", "cleanup-warning", "changed", "recording-off"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "photo.jpg")
			if err := os.WriteFile(path, []byte("photo"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "db.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			before, _ := CaptureUploadRecordSnapshot(path)
			b := &recordBatch{store: s, config: Config{Selected: "a", RecordUploads: mode != "recording-off"}}
			_, _, _, _ = b.run(ctx, UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}}, func(string, any) {}, func(context.Context, ProgressCallback) (string, bool, error) { return "", false, nil })
			_, _, _, _ = b.run(ctx, UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}}, func(string, any) {}, func(ctx context.Context, cb ProgressCallback) (string, bool, error) {
				switch mode {
				case "failure":
					return "", false, errors.New("failed")
				case "remote":
					cb("recordRemoteMatch", nil)
				case "cancel-after-success":
					cancel()
				case "cleanup-warning":
					return "key", false, errors.New("cleanup failed")
				case "changed":
					if err := os.WriteFile(path, []byte("changed-size"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return "key", false, nil
			})
			m, err := s.Match(context.Background(), "a", []UploadRecordSnapshot{before})
			if err != nil {
				t.Fatal(err)
			}
			want := mode != "failure" && mode != "changed" && mode != "recording-off"
			if (len(m) == 1) != want {
				t.Fatalf("recorded=%v want=%v", m, want)
			}
			if mode == "remote" && m[path].Origin != "remote-match" {
				t.Fatal("origin lost")
			}
		})
	}
}

func TestRecordBatchDatabaseErrorAndCancellation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(path, []byte("photo"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	b := &recordBatch{store: s, config: Config{Selected: "a", SkipRecordedUploads: true}}
	calls, warnings := 0, 0
	normal := func(context.Context, ProgressCallback) (string, bool, error) { calls++; return "normal", false, nil }
	cb := func(event string, _ any) {
		if event == "uploadWarning" {
			warnings++
		}
	}
	_, _, local, err := b.run(ctx, UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}}, cb, normal)
	if err != nil || local || calls != 1 || warnings != 1 {
		t.Fatal("database fallback failed")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, _, err = b.run(cancelled, UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}}, cb, normal)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal("cancelled lookup started upload")
	}
	keys := uniqueMediaKeys([]string{"a", "a", "", "b"})
	if len(keys) != 2 {
		t.Fatal(keys)
	}
}

func TestRecordBatchMissingMediaKeyDoesNotTransfer(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(path, []byte("photo"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	before, _ := CaptureUploadRecordSnapshot(path)
	record, _ := ConfirmUploadRecord(before, "key", "transfer", false)
	if err := s.SaveConfirmed(ctx, "a", []UploadRecord{record}); err != nil {
		t.Fatal(err)
	}
	// Simulate a damaged/legacy entry; production writes reject empty media keys.
	if _, err := s.db.Exec("PRAGMA ignore_check_constraints=ON; UPDATE upload_records SET media_key=''"); err != nil {
		t.Fatal(err)
	}
	b := &recordBatch{store: s, config: Config{Selected: "a", SkipRecordedUploads: true}}
	_, _, _, err = b.run(ctx, UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}}, func(string, any) {}, func(context.Context, ProgressCallback) (string, bool, error) {
		t.Fatal("silently retransferred")
		return "", false, nil
	})
	if !errors.Is(err, errRecordedMediaKeyMissing) {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRecordBatchAllAndMixedAlbumCandidates(t *testing.T) {
	for _, allRecorded := range []bool{true, false} {
		t.Run(fmt.Sprint(allRecorded), func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			s, err := OpenUploadRecordStore(ctx, filepath.Join(dir, "db.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			b := &recordBatch{store: s, config: Config{Selected: "a", RecordUploads: true, SkipRecordedUploads: true}}
			var items []UploadWorkItem
			for i := 0; i < 3; i++ {
				folder := filepath.Join(dir, fmt.Sprint(i))
				if err := os.Mkdir(folder, 0700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(folder, "photo.jpg")
				if err := os.WriteFile(path, []byte("photo"), 0600); err != nil {
					t.Fatal(err)
				}
				items = append(items, UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}})
				if allRecorded || i < 2 {
					before, _ := CaptureUploadRecordSnapshot(path)
					r, _ := ConfirmUploadRecord(before, fmt.Sprint(i), "transfer", false)
					if err := s.SaveConfirmed(ctx, "a", []UploadRecord{r}); err != nil {
						t.Fatal(err)
					}
				}
			}
			transfers := 0
			albums := map[string][]string{}
			for _, item := range items {
				key, _, _, err := b.run(ctx, item, func(string, any) {}, func(context.Context, ProgressCallback) (string, bool, error) { transfers++; return "new", false, nil })
				if err != nil {
					t.Fatal(err)
				}
				folder := filepath.Dir(item.Single.Path)
				albums[folder] = append(albums[folder], key)
			}
			want := 1
			if allRecorded {
				want = 0
			}
			if transfers != want || len(albums) != 3 {
				t.Fatalf("transfers=%d folders=%d", transfers, len(albums))
			}
			for _, keys := range albums {
				if len(keys) != 1 || keys[0] == "" {
					t.Fatal("missing album candidate")
				}
			}
		})
	}
}
