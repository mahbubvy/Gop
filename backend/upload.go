package backend

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// FilesDroppedEvent is emitted when files are dropped on any drop zone
type FilesDroppedEvent struct {
	Files    []string `json:"files"`
	DropZone string   `json:"dropZone"`
}

// StartUploadEvent is received from frontend to start upload
type StartUploadEvent struct {
	Files              []string `json:"files"`
	BypassLocalRecords bool     `json:"bypassLocalRecords"`
}

// ProgressCallback is a function type for upload progress updates
type ProgressCallback func(event string, data any)

type UploadManager struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	cancel   chan struct{}
	canceled bool
	running  bool
	app      AppInterface
}

func NewUploadManager(app AppInterface) *UploadManager {
	return &UploadManager{
		app: app,
	}
}

func (m *UploadManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *UploadManager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil && !m.canceled {
		close(m.cancel)
		m.canceled = true
		// Don't set to nil - readers still need to detect closure via select
	}
}

// isCancelled checks if cancellation has been requested
func (m *UploadManager) isCancelled() bool {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel == nil {
		return false
	}
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

// getCancelChan returns the cancel channel safely
func (m *UploadManager) getCancelChan() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancel
}

type UploadBatchStart struct {
	Total            int      `json:"Total"`
	TotalBytes       int64    `json:"TotalBytes"`
	AlbumName        string   `json:"AlbumName"`
	AlbumAutoMode    bool     `json:"AlbumAutoMode"`
	AutoAlbumFolders []string `json:"AutoAlbumFolders"`
	WorkPaths        []string `json:"WorkPaths"`
}

type FileUploadResult struct {
	Cancelled    bool     `json:"Cancelled"`
	MediaKey     string   `json:"MediaKey"`
	IsError      bool     `json:"IsError"`
	IsLivePhoto  bool     `json:"IsLivePhoto"`
	Skipped      bool     `json:"Skipped"`
	SkipCode     string   `json:"SkipCode"`
	SkipReason   string   `json:"SkipReason"`
	Error        error    `json:"-"`
	ErrorMessage string   `json:"ErrorMessage"`
	Path         string   `json:"Path"`
	Paths        []string `json:"Paths"`
}

type ThreadStatus struct {
	WorkerID      int    `json:"WorkerID"`
	Status        string `json:"Status"` // "idle", "hashing", "checking", "uploading", "finalizing", "completed", "skipped", "error"
	FilePath      string `json:"FilePath"`
	FileName      string `json:"FileName"`
	Message       string `json:"Message"`
	BytesUploaded int64  `json:"BytesUploaded"`
	BytesTotal    int64  `json:"BytesTotal"`
	Attempt       int    `json:"Attempt"` // Current attempt number (1-based), 0 if not applicable
}

func (m *UploadManager) Upload(app AppInterface, paths []string, bypassLocalRecords ...bool) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.cancel = make(chan struct{})
	m.canceled = false
	m.mu.Unlock()

	// Make preflight visible immediately so a long directory or metadata scan can
	// be cancelled from the UI.
	configMu.RLock()
	batchConfig := AppConfig
	batchConfig.Credentials = append([]string(nil), AppConfig.Credentials...)
	configMu.RUnlock()
	albumName, albumAutoMode := batchConfig.AlbumName, batchConfig.AlbumAutoMode
	app.EmitEvent("uploadStart", UploadBatchStart{
		AlbumName:     albumName,
		AlbumAutoMode: albumAutoMode,
	})

	targetPaths, err := filterGooglePhotosFilesWithCancel(paths, m.isCancelled, batchConfig)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			m.finishPreflight(app)
			return
		}
		app.EmitEvent("FileStatus", FileUploadResult{
			IsError:      true,
			Error:        err,
			ErrorMessage: err.Error(),
		})
		m.finishPreflight(app)
		return
	}
	workItems, preflightWarnings := ClassifyUploadWork(targetPaths, LivePhotoClassificationOptions{
		Enabled:             batchConfig.PairLivePhotos,
		SkipIncomplete:      batchConfig.SkipIncompleteLivePhotos,
		IgnoreAppleMetadata: batchConfig.IgnoreAppleMetadata,
		Cancelled:           m.isCancelled,
	}, nil)
	if m.isCancelled() {
		m.finishPreflight(app)
		return
	}
	emitUploadPreflight(app, workItems, preflightWarnings, albumName, albumAutoMode)

	if len(workItems) == 0 {
		m.finishPreflight(app)
		return
	}

	if _, err := newAPIForConfig(batchConfig); err != nil {
		for _, item := range workItems {
			path := uploadWorkPrimaryPath(item)
			app.EmitEvent("FileStatus", FileUploadResult{
				IsError:      true,
				Error:        err,
				ErrorMessage: err.Error(),
				Path:         path,
			})
		}
		app.EmitEvent("uploadStop", nil)
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		return
	}

	// Publish before workers so local-skip deltas cannot race the initial total.
	func() {
		var totalBytes int64
		for _, item := range workItems {
			if m.isCancelled() {
				return
			}
			for _, path := range uploadWorkPaths(item) {
				if info, err := os.Stat(path); err == nil {
					totalBytes += info.Size()
				}
			}
		}
		app.EmitEvent("uploadTotalBytes", totalBytes)
	}()
	if m.isCancelled() {
		m.finishPreflight(app)
		return
	}
	records := &recordBatch{config: batchConfig, bypass: len(bypassLocalRecords) > 0 && bypassLocalRecords[0]}
	if batchConfig.RecordUploads || batchConfig.SkipRecordedUploads {
		path, openErr := DefaultUploadRecordPath()
		if openErr == nil {
			records.store, openErr = OpenUploadRecordStore(context.Background(), path)
		}
		if openErr != nil {
			app.EmitEvent("uploadWarning", PreflightWarning{Code: "local-record-error", Message: "Local upload records unavailable; proceeding with normal uploads."})
		}
	}

	// Don't start more threads than files to process
	numWorkers := min(max(1, batchConfig.UploadThreads), len(workItems))

	// Create a worker pool for concurrent uploads
	workChan := make(chan UploadWorkItem, len(workItems))
	results := make(chan FileUploadResult, len(workItems))

	// Start workers
	for i := range numWorkers {
		m.wg.Add(1)
		go startUploadWorker(i, workChan, results, m.cancel, &m.wg, app, records)
	}

	// Send work to workers
	go func() {
	LOOP:
		for _, item := range workItems {
			select {
			case <-m.cancel:
				break LOOP
			case workChan <- item:
			}
		}
		close(workChan)
	}()

	// Handle results, wait for completion, and create album if configured
	go func() {
		// Collect successful uploads with path -> mediaKey mapping for AUTO mode
		if records.store != nil {
			defer records.store.Close()
		}
		successfulUploads := make(map[string]string) // path -> mediaKey

		// Wait for all workers to finish in a separate goroutine, then close results
		go func() {
			m.wg.Wait()
			close(results)
		}()

		// Process all results (this blocks until results channel is closed)
		for result := range results {
			app.EmitEvent("FileStatus", result)
			if result.IsError {
				s := fmt.Sprintf("upload error: %v", result.Error)
				app.GetLogger().Error(s)
			} else {
				s := fmt.Sprintf("upload success: %v", result.Path)
				app.GetLogger().Info(s)
				if result.MediaKey != "" {
					successfulUploads[result.Path] = result.MediaKey
				}
			}
		}

		// Handle album creation after all results are processed
		// Use the same captured account and destination as the workers.
		app.GetLogger().Info(fmt.Sprintf("Upload complete. Successful uploads: %d, AlbumName: '%s', AlbumAutoMode: %v",
			len(successfulUploads), albumName, albumAutoMode))

		if len(successfulUploads) > 0 {
			m.handleAlbumCreation(app, successfulUploads, albumName, albumAutoMode, batchConfig)
		}

		app.EmitEvent("uploadStop", nil)
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()
}

func (m *UploadManager) finishPreflight(app AppInterface) {
	app.EmitEvent("uploadStop", nil)
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
}

func emitUploadPreflight(app AppInterface, workItems []UploadWorkItem, warnings []PreflightWarning, albumName string, albumAutoMode bool) {
	workPaths := make([]string, 0, len(workItems))
	for _, item := range workItems {
		workPaths = append(workPaths, uploadWorkPrimaryPath(item))
	}
	skippedCount := 0
	for _, warning := range warnings {
		if isSkippedPreflightWarning(warning.Code) {
			skippedCount++
		}
	}

	// Start before emitting warnings/results so the frontend resets its previous
	// batch while retaining every event from this preflight.
	app.EmitEvent("uploadStart", UploadBatchStart{
		Total:            len(workItems) + skippedCount,
		TotalBytes:       0,
		AlbumName:        albumName,
		AlbumAutoMode:    albumAutoMode,
		AutoAlbumFolders: autoAlbumFolders(workItems, albumAutoMode),
		WorkPaths:        workPaths,
	})
	for _, warning := range warnings {
		app.EmitEvent("uploadWarning", warning)
		if !isSkippedPreflightWarning(warning.Code) {
			continue
		}
		primaryPath := ""
		if len(warning.Paths) > 0 {
			primaryPath = warning.Paths[0]
		}
		app.EmitEvent("FileStatus", FileUploadResult{
			IsLivePhoto: true,
			Skipped:     true,
			SkipCode:    warning.Code,
			SkipReason:  warning.Message,
			Path:        primaryPath,
			Paths:       warning.Paths,
		})
	}
}

// autoAlbumFolders returns each source directory once, so the upload view can
// show every Auto Album destination before the media transfer finishes. Album
// creation itself still uses the original path-to-media-key mapping unchanged.
func autoAlbumFolders(workItems []UploadWorkItem, autoMode bool) []string {
	if !autoMode {
		return nil
	}

	folders := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range workItems {
		folder := filepath.Clean(filepath.Dir(uploadWorkPrimaryPath(item)))
		if folder == "." || folder == "" {
			continue
		}
		if _, exists := seen[folder]; exists {
			continue
		}
		seen[folder] = struct{}{}
		folders = append(folders, folder)
	}
	return folders
}

func isSkippedPreflightWarning(code string) bool {
	return code == "incomplete-live-photo-skipped" || code == "ambiguous-filename-stem"
}

// handleAlbumCreation handles album creation based on config (manual name/key or AUTO mode)
func (m *UploadManager) handleAlbumCreation(app AppInterface, uploads map[string]string, albumName string, albumAutoMode bool, configs ...Config) {
	// Check if cancelled before starting album creation
	if m.isCancelled() {
		app.GetLogger().Info("Upload cancelled, skipping album creation")
		return
	}

	app.GetLogger().Info(fmt.Sprintf("handleAlbumCreation called with %d uploads", len(uploads)))

	// Create API once for all album operations
	config := uploadConfig(context.Background())
	if len(configs) > 0 {
		config = configs[0]
	}
	api, err := newAPIForConfig(config)
	if err != nil {
		app.GetLogger().Error(fmt.Sprintf("failed to create API for album creation: %v", err))
		app.EmitEvent("albumError", AlbumError{
			AlbumName: albumName,
			Error:     fmt.Sprintf("failed to initialize API: %v", err),
		})
		return
	}

	albumManager := NewAlbumManager(api, app, m.getCancelChan())

	// Check if AUTO mode is enabled
	if albumAutoMode {
		app.GetLogger().Info("AUTO mode enabled, creating albums from directories")
		m.createAlbumsFromDirectories(albumManager, app, uploads)
		return
	}

	// Manual mode: use AlbumName if set
	if albumName == "" {
		app.GetLogger().Info("No album name set and AUTO mode disabled, skipping album creation")
		return
	}

	app.GetLogger().Info(fmt.Sprintf("Creating album with name/key: '%s'", albumName))

	mediaKeys := make([]string, 0, len(uploads))
	for _, mediaKey := range uploads {
		mediaKeys = append(mediaKeys, mediaKey)
	}

	app.GetLogger().Info(fmt.Sprintf("Adding %d media keys to album '%s'", len(mediaKeys), albumName))

	mediaKeys = uniqueMediaKeys(mediaKeys)
	albumKeys, err := albumManager.AddToAlbum(mediaKeys, albumName)
	if err != nil {
		app.GetLogger().Error(fmt.Sprintf("failed to create album '%s': %v", albumName, err))
		app.EmitEvent("albumError", AlbumError{
			AlbumName: albumName,
			Error:     err.Error(),
		})
		return
	}
	app.GetLogger().Info(fmt.Sprintf("created album '%s' with %d items, album keys: %v", albumName, len(mediaKeys), albumKeys))
}

// createAlbumsFromDirectories creates albums based on parent directory names (AUTO mode)
func (m *UploadManager) createAlbumsFromDirectories(albumManager *AlbumManager, app AppInterface, uploads map[string]string) {
	// Group media keys by parent directory
	mediaKeysByDir := make(map[string][]string)

	for filePath, mediaKey := range uploads {
		parentDir := filepath.Dir(filePath)
		mediaKeysByDir[parentDir] = append(mediaKeysByDir[parentDir], mediaKey)
	}

	// Create an album for each directory
	for dirPath, mediaKeys := range mediaKeysByDir {
		mediaKeys = uniqueMediaKeys(mediaKeys)
		albumName := filepath.Base(dirPath)
		if albumName == "" || albumName == "." {
			albumName = "Uploads"
		}

		albumKeys, err := albumManager.AddToAlbum(mediaKeys, albumName)
		if err != nil {
			app.GetLogger().Error(fmt.Sprintf("failed to create album '%s': %v", albumName, err))
			app.EmitEvent("albumError", AlbumError{
				AlbumName: albumName,
				Error:     err.Error(),
			})
			continue
		}
		app.GetLogger().Info(fmt.Sprintf("created album '%s' with %d items, album keys: %v", albumName, len(mediaKeys), albumKeys))
	}
}

// supportedFormats is a map of file extensions supported by Google Photos (O(1) lookup)
var supportedFormats = map[string]bool{
	// Photo formats
	"avif": true, "bmp": true, "gif": true, "heic": true, "heif": true, "ico": true,
	"jpg": true, "jpeg": true, "png": true, "tif": true, "tiff": true, "webp": true,
	"cr2": true, "cr3": true, "nef": true, "arw": true, "orf": true,
	"raf": true, "rw2": true, "pef": true, "sr2": true, "dng": true,
	// Video formats
	"3gp": true, "3g2": true, "asf": true, "avi": true, "divx": true,
	"m2t": true, "m2ts": true, "m4v": true, "mkv": true, "mmv": true,
	"mod": true, "mov": true, "mp4": true, "mpg": true, "mpeg": true,
	"mts": true, "tod": true, "wmv": true, "ts": true, "webm": true,
}

// isSupportedByGooglePhotos checks if a file extension is supported by Google Photos
func isSupportedByGooglePhotos(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}
	// Remove the dot and check map
	return supportedFormats[ext[1:]]
}

func scanDirectoryForFiles(path string, recursive bool, excludePattern string, cancelled func() bool) ([]string, error) {
	var files []string

	if cancelled != nil && cancelled() {
		return nil, context.Canceled
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if cancelled != nil && cancelled() {
			return nil, context.Canceled
		}
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if excludePattern != "" && entry.Name() == excludePattern {
				continue
			}
			if recursive {
				subFiles, err := scanDirectoryForFiles(fullPath, recursive, excludePattern, cancelled)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return nil, err
					}
					continue
				}
				files = append(files, subFiles...)
			}
		} else {
			files = append(files, fullPath)
		}
	}

	return files, nil
}

// FilterGooglePhotosFiles returns a list of files that are supported by Google Photos (exported)
func FilterGooglePhotosFiles(paths []string) ([]string, error) {
	return filterGooglePhotosFiles(paths)
}

// CountUploadCandidates uses the existing scanner without network requests.
func (c *ConfigManager) CountUploadCandidates(paths []string) (int, error) {
	files, err := filterGooglePhotosFiles(paths)
	return len(files), err
}

// PreviewAutoAlbums scans local sources without uploading or making API calls.
func (c *ConfigManager) PreviewAutoAlbums(paths []string) ([]string, error) {
	files, err := filterGooglePhotosFiles(paths)
	if err != nil {
		return nil, err
	}
	items, _ := ClassifyUploadWork(files, LivePhotoClassificationOptions{
		Enabled:             AppConfig.PairLivePhotos,
		SkipIncomplete:      AppConfig.SkipIncompleteLivePhotos,
		IgnoreAppleMetadata: AppConfig.IgnoreAppleMetadata,
	}, nil)
	return autoAlbumFolders(items, true), nil
}

// filterGooglePhotosFiles returns a list of files that are supported by Google Photos
func filterGooglePhotosFiles(paths []string) ([]string, error) {
	return filterGooglePhotosFilesWithCancel(paths, nil)
}

func filterGooglePhotosFilesWithCancel(paths []string, cancelled func() bool, configs ...Config) ([]string, error) {
	config := uploadConfig(context.Background())
	if len(configs) > 0 {
		config = configs[0]
	}
	var supportedFiles []string
	type seenUploadFile struct {
		canonicalPath string
		info          os.FileInfo
	}
	seenFiles := make(map[string][]seenUploadFile)
	appendFile := func(path string, info os.FileInfo) {
		if !config.DisableUnsupportedFilesFilter && !isSupportedByGooglePhotos(path) {
			return
		}
		canonicalPath := canonicalUploadPath(path)
		bucketKey := canonicalPath
		if runtime.GOOS == "windows" {
			bucketKey = strings.ToLower(bucketKey)
		}
		if info == nil {
			info, _ = os.Stat(path)
		}
		for _, seen := range seenFiles[bucketKey] {
			if canonicalPath == seen.canonicalPath ||
				(info != nil && seen.info != nil && os.SameFile(info, seen.info)) {
				return
			}
		}
		seenFiles[bucketKey] = append(seenFiles[bucketKey], seenUploadFile{
			canonicalPath: canonicalPath,
			info:          info,
		})
		supportedFiles = append(supportedFiles, path)
	}

	for _, path := range paths {
		if cancelled != nil && cancelled() {
			return nil, context.Canceled
		}
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing path %s: %w", path, err)
		}

		if fileInfo.IsDir() {
			files, err := scanDirectoryForFiles(path, config.Recursive, config.ExcludePattern, cancelled)
			if err != nil {
				return nil, fmt.Errorf("error scanning directory %s: %w", path, err)
			}

			for _, file := range files {
				if cancelled != nil && cancelled() {
					return nil, context.Canceled
				}
				appendFile(file, nil)
			}
		} else {
			appendFile(path, fileInfo)
		}
	}

	return supportedFiles, nil
}

func canonicalUploadPath(path string) string {
	canonicalPath, err := filepath.Abs(path)
	if err != nil {
		canonicalPath = filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(canonicalPath); err == nil {
		canonicalPath = resolved
	}
	return filepath.Clean(canonicalPath)
}

// UploadFile is an exported version for CLI use with callback
func UploadFile(ctx context.Context, api *Api, filePath string, workerID int, callback ProgressCallback) (string, error) {
	return uploadFileWithCallback(ctx, api, filePath, workerID, callback)
}

func uploadFileWithCallback(ctx context.Context, api *Api, filePath string, workerID int, callback ProgressCallback) (string, error) {
	fileName := filepath.Base(filePath)
	mediakey := ""

	// Determine the timestamp to use for the upload.
	// Default to file mtime, but allow filename-based timestamp to take precedence if enabled.
	var uploadTimestamp int64
	if info, err := os.Stat(filePath); err == nil {
		uploadTimestamp = info.ModTime().Unix()
	}
	if uploadConfig(ctx).SetDateFromFilename {
		if t, ok := parseTimestampFromFilename(filePath); ok {
			uploadTimestamp = t.Unix()
		}
	}

	// Stage 1: Hashing
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "hashing",
		FilePath: filePath,
		FileName: fileName,
		Message:  "Hashing...",
	})

	sha1_hash_bytes, err := CalculateSHA1(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("error calculating hash file: %w", err)
	}

	sha1_hash_b64 := base64.StdEncoding.EncodeToString([]byte(sha1_hash_bytes))

	// Stage 2: Checking if exists in library
	if !uploadConfig(ctx).ForceUpload {
		callback("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "checking",
			FilePath: filePath,
			FileName: fileName,
			Message:  "Checking if file exists in library...",
		})

		mediakey, err = api.FindRemoteMediaByHash(sha1_hash_bytes)
		if err != nil {
			// Non-fatal: log via callback and continue with upload
			callback("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "checking",
				FilePath: filePath,
				FileName: fileName,
				Message:  fmt.Sprintf("Hash check warning: %v, proceeding with upload", err),
			})
		}
		if len(mediakey) > 0 {
			callback("recordRemoteMatch", nil)
			callback("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "completed",
				FilePath: filePath,
				FileName: fileName,
				Message:  "Already in library",
			})
			if uploadConfig(ctx).DeleteFromHost {
				if err := os.Remove(filePath); err != nil {
					return mediakey, fmt.Errorf("file exists in library but failed to delete local copy: %w", err)
				}
			}
			return mediakey, nil
		}
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("error getting file info: %w", err)
	}

	// Stage 3: Uploading
	fileSize := fileInfo.Size()
	callback("ThreadStatus", ThreadStatus{
		WorkerID:      workerID,
		Status:        "uploading",
		FilePath:      filePath,
		FileName:      fileName,
		Message:       "Uploading...",
		BytesUploaded: 0,
		BytesTotal:    fileSize,
	})

	token, err := api.GetUploadToken(sha1_hash_b64, fileSize)
	if err != nil {
		return "", fmt.Errorf("error uploading file: %w", err)
	}

	// Create progress callback for upload
	progressCallback := func(bytesUploaded, bytesTotal int64, attempt int) {
		message := "Uploading..."
		if attempt > 1 {
			message = fmt.Sprintf("Retrying... (attempt %d)", attempt)
		}
		callback("ThreadStatus", ThreadStatus{
			WorkerID:      workerID,
			Status:        "uploading",
			FilePath:      filePath,
			FileName:      fileName,
			Message:       message,
			BytesUploaded: bytesUploaded,
			BytesTotal:    bytesTotal,
			Attempt:       attempt,
		})
	}

	finalizeToken, err := api.UploadFileWithProgress(ctx, filePath, token, progressCallback)
	if err != nil {
		return "", fmt.Errorf("error uploading file: %w", err)
	}
	commitToken, err := finalizeToken.legacyCommitToken()
	if err != nil {
		return "", fmt.Errorf("error decoding upload finalize token: %w", err)
	}

	// Stage 4: Finalizing
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "finalizing",
		FilePath: filePath,
		FileName: fileName,
		Message:  "Committing upload...",
	})

	mediaKey, err := api.CommitUpload(commitToken, fileInfo.Name(), sha1_hash_bytes, uploadTimestamp)
	if err != nil {
		return "", fmt.Errorf("error committing file: %w", err)
	}

	if len(mediaKey) == 0 {
		return "", fmt.Errorf("media key not received")
	}

	if uploadConfig(ctx).DeleteFromHost {
		if err := os.Remove(filePath); err != nil {
			return mediaKey, fmt.Errorf("uploaded successfully but failed to delete file: %w", err)
		}
	}

	return mediaKey, nil
}

func startUploadWorker(workerID int, workChan <-chan UploadWorkItem, results chan<- FileUploadResult, cancel <-chan struct{}, wg *sync.WaitGroup, app AppInterface, records *recordBatch) {
	defer wg.Done()

	// Emit idle status initially
	app.EmitEvent("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Waiting for files...",
	})

	// Create API client once per worker for connection reuse
	api, err := newAPIForConfig(records.config)
	if err != nil {
		app.EmitEvent("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "error",
			Message:  fmt.Sprintf("Failed to initialize API: %v", err),
		})
		return
	}

	// Create callback from app interface (reuse for all files)
	callback := func(event string, data any) {
		if event == "recordRemoteMatch" {
			return
		}
		app.EmitEvent(event, data)
	}

	for item := range workChan {
		select {
		case <-cancel:
			app.EmitEvent("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Cancelled",
			})
			return // Stop if cancellation is requested
		default:
			ctx, cancelUpload := context.WithCancel(context.Background())
			go func() {
				select {
				case <-cancel:
					cancelUpload()
				case <-ctx.Done():
					// Upload completed or context cancelled
				}
			}()

			path := uploadWorkPrimaryPath(item)
			paths := uploadWorkPaths(item)
			isLivePhoto := item.Kind == UploadWorkLivePhoto
			mediaKey, skipped, localMatch, err := records.run(ctx, item, callback, func(ctx context.Context, cb ProgressCallback) (string, bool, error) {
				return uploadWorkItem(ctx, api, item, workerID, cb)
			})
			if err != nil && mediaKey != "" {
				results <- FileUploadResult{IsLivePhoto: isLivePhoto, Path: path, Paths: paths, MediaKey: mediaKey}
				app.EmitEvent("uploadWarning", PreflightWarning{
					Paths:   paths,
					Code:    "local-cleanup-failed",
					Message: err.Error(),
				})
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "completed",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("Uploaded, but local cleanup failed: %v", err),
				})
			} else if err != nil {
				results <- FileUploadResult{IsError: true, Cancelled: errors.Is(err, context.Canceled), IsLivePhoto: isLivePhoto, Error: err, ErrorMessage: err.Error(), Path: path, Paths: paths}
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "error",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("Error: %v", err),
				})
			} else if skipped {
				skipCode := "remote-duplicate"
				skipReason := "Skipped because this file already exists remotely"
				if isLivePhoto {
					skipCode = "remote-live-photo-component-exists"
					skipReason = "Skipped because a Live Photo component already exists remotely"
				}
				if localMatch {
					skipCode = "local-record"
					skipReason = "Previously uploaded on this PC; saved media reused for any selected album"
				}
				results <- FileUploadResult{
					MediaKey:    mediaKey,
					IsLivePhoto: isLivePhoto,
					Skipped:     true,
					SkipCode:    skipCode,
					SkipReason:  skipReason,
					Path:        path,
					Paths:       paths,
				}
			} else {
				results <- FileUploadResult{IsLivePhoto: isLivePhoto, Path: path, Paths: paths, MediaKey: mediaKey}
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "completed",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  "Completed",
				})
			}
			cancelUpload()

			// Mark as idle after completing file
			app.EmitEvent("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Waiting for next file...",
			})
		}
	}

	// Final idle status when no more work
	app.EmitEvent("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Finished",
	})
}

func uploadWorkItem(ctx context.Context, api *Api, item UploadWorkItem, workerID int, callback ProgressCallback) (string, bool, error) {
	switch item.Kind {
	case UploadWorkSingle:
		if item.Single == nil || item.LivePhoto != nil {
			return "", false, fmt.Errorf("invalid single-media work item")
		}
		mediaKey, err := uploadFileWithCallback(ctx, api, item.Single.Path, workerID, callback)
		return mediaKey, false, err
	case UploadWorkLivePhoto:
		if item.LivePhoto == nil || item.Single != nil {
			return "", false, fmt.Errorf("invalid Live Photo work item")
		}
		return uploadLivePhotoWithCallback(ctx, api, *item.LivePhoto, LivePhotoUploadOptions{
			Policy:                     buildLivePhotoCommitPolicy(api, uploadConfig(ctx)),
			DeleteFromHost:             uploadConfig(ctx).DeleteFromHost,
			SetDateFromFilename:        uploadConfig(ctx).SetDateFromFilename,
			UpdateExistingPhotosToLive: uploadConfig(ctx).UpdateExistingPhotosToLive,
		}, workerID, callback)
	default:
		return "", false, fmt.Errorf("unsupported upload work kind %q", item.Kind)
	}
}

func uploadWorkPaths(item UploadWorkItem) []string {
	if item.Kind == UploadWorkLivePhoto && item.LivePhoto != nil {
		return []string{item.LivePhoto.PhotoPath, item.LivePhoto.VideoPath}
	}
	if item.Single != nil {
		return []string{item.Single.Path}
	}
	return nil
}

func uploadWorkPrimaryPath(item UploadWorkItem) string {
	paths := uploadWorkPaths(item)
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}
