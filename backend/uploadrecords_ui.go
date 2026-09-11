package backend

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type UploadRecordRow struct {
	Path       string `json:"path"`
	Folder     string `json:"folder"`
	Size       int64  `json:"size"`
	RecordedAt string `json:"recordedAt"`
	Origin     string `json:"origin"`
}
type UploadRecordPage struct {
	Rows    []UploadRecordRow `json:"rows"`
	HasMore bool              `json:"hasMore"`
}

func (s *UploadRecordStore) page(ctx context.Context, account, search string, offset int) (UploadRecordPage, error) {
	result := UploadRecordPage{Rows: []UploadRecordRow{}}
	if account == "" || offset < 0 {
		return result, errors.New("invalid record query")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT path,size,recorded_at_ns,origin FROM upload_records WHERE account=? AND instr(lower(path),lower(?))>0 ORDER BY path LIMIT 51 OFFSET ?`, account, search, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var r UploadRecordRow
		var ns int64
		if err := rows.Scan(&r.Path, &r.Size, &ns, &r.Origin); err != nil {
			return result, err
		}
		r.Folder = filepath.Dir(r.Path)
		r.RecordedAt = time.Unix(0, ns).UTC().Format(time.RFC3339)
		result.Rows = append(result.Rows, r)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if len(result.Rows) > 50 {
		result.HasMore = true
		result.Rows = result.Rows[:50]
	}
	return result, nil
}

func openSelectedRecordStore(account string) (*UploadRecordStore, error) {
	ensureConfigLoaded()
	configMu.RLock()
	selected := AppConfig.Selected
	configMu.RUnlock()
	if account == "" || account != selected {
		return nil, errors.New("account changed; reopen Upload Records")
	}
	path, err := DefaultUploadRecordPath()
	if err != nil {
		return nil, err
	}
	return OpenUploadRecordStore(context.Background(), path)
}

func (g *ConfigManager) GetUploadRecords(account, search string, offset int) (UploadRecordPage, error) {
	s, err := openSelectedRecordStore(account)
	if err != nil {
		return UploadRecordPage{}, err
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.page(ctx, account, search, offset)
}

func (s *UploadRecordStore) forget(ctx context.Context, account string, paths []string) error {
	if account == "" || len(paths) > UploadRecordBatchLimit {
		return errors.New("invalid record selection")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, path := range paths {
		if _, err := tx.ExecContext(ctx, "DELETE FROM upload_records WHERE account=? AND path=?", account, path); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (g *ConfigManager) ForgetUploadRecords(account string, paths []string) error {
	s, err := openSelectedRecordStore(account)
	if err != nil {
		return err
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.forget(ctx, account, paths)
}

func safeCSVCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if (len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0]))) || strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") {
		return "'" + value
	}
	return value
}

// Export is streamed and excludes saved remote media keys and credentials.
func (g *ConfigManager) ExportUploadRecords(account, path string) error {
	s, err := openSelectedRecordStore(account)
	if err != nil {
		return err
	}
	defer s.Close()
	if !filepath.IsAbs(path) || strings.ToLower(filepath.Ext(path)) != ".csv" {
		return errors.New("choose a .csv file")
	}
	dbPath, _ := DefaultUploadRecordPath()
	if strings.EqualFold(filepath.Clean(path), filepath.Clean(dbPath)) || strings.EqualFold(filepath.Clean(path), filepath.Clean(ConfigPath)) {
		return errors.New("cannot overwrite application data")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".gotohp-export-*.csv")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	defer file.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.exportCSV(ctx, account, file); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func (s *UploadRecordStore) exportCSV(ctx context.Context, account string, output io.Writer) error {
	rows, err := s.db.QueryContext(ctx, "SELECT path,size,recorded_at_ns,origin FROM upload_records WHERE account=? ORDER BY path", account)
	if err != nil {
		return err
	}
	defer rows.Close()
	w := csv.NewWriter(output)
	if err := w.Write([]string{"Account", "Folder", "File path", "Size bytes", "Recorded at UTC", "Source"}); err != nil {
		return err
	}
	for rows.Next() {
		var p, origin string
		var size, ns int64
		if err := rows.Scan(&p, &size, &ns, &origin); err != nil {
			return err
		}
		if err := w.Write([]string{safeCSVCell(account), safeCSVCell(filepath.Dir(p)), safeCSVCell(p), fmt.Sprint(size), time.Unix(0, ns).UTC().Format(time.RFC3339), origin}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return nil
}

type UploadRecordPreview struct {
	Total      int  `json:"total"`
	Matched    int  `json:"matched"`
	UsesAlbums bool `json:"usesAlbums"`
}

var recordPreviewMu sync.Mutex

type recordPreviewRequest struct {
	cancel    context.CancelFunc
	cancelled bool
	created   time.Time
}

var recordPreviewRequests = map[string]recordPreviewRequest{}

func (g *ConfigManager) CancelUploadRecordPreview(id string) {
	recordPreviewMu.Lock()
	defer recordPreviewMu.Unlock()
	request := recordPreviewRequests[id]
	if request.cancel != nil {
		request.cancel()
	}
	request.cancelled = true
	request.created = time.Now()
	recordPreviewRequests[id] = request
	pruneRecordPreviews()
}

// Called under the mutex; tombstones handle cancellation before dispatch arrives.
func pruneRecordPreviews() {
	for id, request := range recordPreviewRequests {
		if time.Since(request.created) > 3*time.Minute {
			if request.cancel != nil {
				request.cancel()
			}
			delete(recordPreviewRequests, id)
		}
	}
	if len(recordPreviewRequests) > 64 {
		for id, request := range recordPreviewRequests {
			if request.cancel == nil {
				delete(recordPreviewRequests, id)
				break
			}
		}
	}
}

func (g *ConfigManager) PreviewUploadRecords(paths []string, id string) (UploadRecordPreview, error) {
	ensureConfigLoaded()
	config := uploadConfig(context.Background())
	result := UploadRecordPreview{UsesAlbums: config.AlbumAutoMode || config.AlbumName != ""}
	if !config.SkipRecordedUploads || config.ForceUpload || config.DeleteFromHost {
		return result, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	recordPreviewMu.Lock()
	pruneRecordPreviews()
	prior := recordPreviewRequests[id]
	if prior.cancelled {
		cancel()
	}
	if prior.cancel != nil {
		prior.cancel()
	}
	recordPreviewRequests[id] = recordPreviewRequest{cancel: cancel, created: time.Now()}
	recordPreviewMu.Unlock()
	defer func() { recordPreviewMu.Lock(); delete(recordPreviewRequests, id); recordPreviewMu.Unlock() }()
	s, err := openSelectedRecordStore(config.Selected)
	if err != nil {
		return result, err
	}
	defer s.Close()
	return s.preview(ctx, paths, config)
}

func (s *UploadRecordStore) preview(ctx context.Context, paths []string, config Config) (UploadRecordPreview, error) {
	result := UploadRecordPreview{UsesAlbums: config.AlbumAutoMode || config.AlbumName != ""}
	files, err := filterGooglePhotosFilesWithCancel(paths, func() bool { return ctx.Err() != nil }, config)
	if err != nil {
		return result, err
	}
	items, _ := ClassifyUploadWork(files, LivePhotoClassificationOptions{Enabled: config.PairLivePhotos, SkipIncomplete: config.SkipIncompleteLivePhotos, IgnoreAppleMetadata: config.IgnoreAppleMetadata, Cancelled: func() bool { return ctx.Err() != nil }}, nil)
	result.Total = len(items)
	batch := make([]UploadRecordSnapshot, 0, UploadRecordBatchLimit)
	flush := func() error {
		matches, err := s.Match(ctx, config.Selected, batch)
		if err != nil {
			return err
		}
		result.Matched += len(matches)
		batch = batch[:0]
		return nil
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if item.Kind != UploadWorkSingle || item.Single == nil {
			continue
		}
		snapshot, err := CaptureUploadRecordSnapshot(item.Single.Path)
		if err != nil {
			return result, err
		}
		batch = append(batch, snapshot)
		if len(batch) == UploadRecordBatchLimit {
			if err := flush(); err != nil {
				return result, err
			}
		}
	}
	if err := flush(); err != nil {
		return result, err
	}
	return result, ctx.Err()
}
