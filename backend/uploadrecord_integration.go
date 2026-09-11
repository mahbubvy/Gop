package backend

import (
	"context"
	"errors"
	"time"
)

type uploadConfigKey struct{}

func uploadConfig(ctx context.Context) Config {
	if config, ok := ctx.Value(uploadConfigKey{}).(Config); ok {
		return config
	}
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig
}

type recordBatch struct {
	store  *UploadRecordStore
	config Config
	bypass bool
}

// This wrapper leaves the normal transport unchanged. Writes are synchronous,
// so cancellation cannot strand a buffered success awaiting a later flush.
func (b *recordBatch) run(ctx context.Context, item UploadWorkItem, callback ProgressCallback,
	normal func(context.Context, ProgressCallback) (string, bool, error)) (string, bool, bool, error) {
	ctx = context.WithValue(ctx, uploadConfigKey{}, b.config)
	if b.store == nil || item.Kind != UploadWorkSingle || item.Single == nil {
		key, skipped, err := normal(ctx, callback)
		return key, skipped, false, err
	}
	warn := func() {
		callback("uploadWarning", PreflightWarning{Paths: uploadWorkPaths(item), Code: "local-record-error", Message: "Local upload record could not be checked or saved. No local skip was assumed."})
	}
	before, statErr := CaptureUploadRecordSnapshot(item.Single.Path)
	if statErr != nil {
		warn()
	}
	if statErr == nil && b.config.SkipRecordedUploads && !b.config.ForceUpload && !b.config.DeleteFromHost && !b.bypass {
		matches, err := b.store.Match(ctx, b.config.Selected, []UploadRecordSnapshot{before})
		if err != nil {
			if errors.Is(err, errRecordedMediaKeyMissing) {
				return "", false, false, err
			}
			if ctx.Err() != nil {
				return "", false, false, ctx.Err()
			}
			warn()
		} else if record, ok := matches[before.Path]; ok {
			if err := ctx.Err(); err != nil {
				return "", false, false, err
			}
			callback("uploadTotalBytesDelta", -before.Size)
			return record.MediaKey, true, true, nil
		}
	}
	origin := "transfer"
	wrapped := func(event string, data any) {
		if event == "recordRemoteMatch" {
			origin = "remote-match"
			return
		}
		callback(event, data)
	}
	key, skipped, err := normal(ctx, wrapped)
	if b.config.RecordUploads && statErr == nil && key != "" && !skipped {
		record, confirmErr := ConfirmUploadRecord(before, key, origin, b.config.DeleteFromHost && err == nil)
		if confirmErr == nil {
			flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			confirmErr = b.store.SaveConfirmed(flushCtx, b.config.Selected, []UploadRecord{record})
			cancel()
		}
		if confirmErr != nil {
			warn()
		}
	}
	return key, skipped, false, err
}

func uniqueMediaKeys(keys []string) []string {
	seen := make(map[string]bool, len(keys))
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if key != "" && !seen[key] {
			seen[key] = true
			result = append(result, key)
		}
	}
	return result
}
