package backend

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	_ "modernc.org/sqlite"
)

type Config struct {
	RecordUploads                 bool                           `json:"recordUploads" koanf:"record_uploads"`
	SkipRecordedUploads           bool                           `json:"skipRecordedUploads" koanf:"skip_recorded_uploads"`
	Credentials                   []string                       `json:"credentials,omitempty" koanf:"credentials"`
	Selected                      string                         `json:"selected" koanf:"selected"`
	Proxy                         string                         `json:"proxy" koanf:"proxy"`
	UseQuota                      bool                           `json:"useQuota" koanf:"use_quota"`
	Saver                         bool                           `json:"saver" koanf:"saver"`
	Recursive                     bool                           `json:"recursive" koanf:"recursive"`
	ForceUpload                   bool                           `json:"forceUpload" koanf:"force_upload"`
	PairLivePhotos                bool                           `json:"pairLivePhotos" koanf:"pair_live_photos"`
	SkipIncompleteLivePhotos      bool                           `json:"skipIncompleteLivePhotos" koanf:"skip_incomplete_live_photos"`
	UpdateExistingPhotosToLive    bool                           `json:"updateExistingPhotosToLive" koanf:"update_existing_photos_to_live"`
	UploadThreads                 int                            `json:"uploadThreads" koanf:"upload_threads"`
	DeleteFromHost                bool                           `json:"deleteFromHost" koanf:"delete_from_host"`
	DisableUnsupportedFilesFilter bool                           `json:"disableUnsupportedFilesFilter" koanf:"disable_unsupported_files_filter"`
	AlbumName                     string                         `json:"albumName" koanf:"album_name"`
	AlbumAutoMode                 bool                           `json:"albumAutoMode" koanf:"album_auto_mode"`
	RecentAlbumsByAccount         map[string][]RecentAlbum       `json:"recentAlbumsByAccount,omitempty" koanf:"recent_albums_by_account"`
	AlbumHistoryByAccount         map[string][]AlbumHistoryEntry `json:"albumHistoryByAccount,omitempty" koanf:"album_history_by_account"`
	SetDateFromFilename           bool                           `json:"setDateFromFilename" koanf:"set_date_from_filename"`
	ExcludePattern                string                         `json:"excludePattern" koanf:"exclude_pattern"`
	// IgnoreAppleMetadata is a CLI-only per-command override and is never persisted.
	IgnoreAppleMetadata bool `json:"-" koanf:"-"`
}

// RecentAlbum is a locally stored shortcut to an album that was successfully
// updated by this app. AlbumID is the authoritative Google Photos album key.
type RecentAlbum struct {
	AlbumID    string `json:"albumId" koanf:"album_id"`
	AlbumName  string `json:"albumName" koanf:"album_name"`
	LastUsedAt int64  `json:"lastUsedAt" koanf:"last_used_at"`
}

const maxRecentAlbumsPerAccount = 14
const maxAlbumHistoryPerAccount = 40

type AlbumHistoryEntry struct {
	AlbumID    string `json:"albumId" koanf:"album_id"`
	AlbumName  string `json:"albumName" koanf:"album_name"`
	ItemsAdded int    `json:"itemsAdded" koanf:"items_added"`
	Operation  string `json:"operation" koanf:"operation"`
	CreatedAt  int64  `json:"createdAt" koanf:"created_at"`
}

type AccountSummary struct {
	Email             string `json:"email"`
	NeedsTokenBinding bool   `json:"needsTokenBinding"`
}

type AccountsState struct {
	Accounts []AccountSummary `json:"accounts"`
	Selected string           `json:"selected"`
}

type ConfigManager struct{}

var (
	configMu      sync.RWMutex
	AppConfig     Config
	UploadRunning bool = false
	ConfigPath    string
	DefaultConfig = Config{
		RecordUploads:            true,
		SkipIncompleteLivePhotos: true,
		UploadThreads:            3,
	}
)

// ParseAuthString parses an auth string and returns url.Values (exported for CLI use)
func ParseAuthString(authString string) (url.Values, error) {
	return url.ParseQuery(authString)
}

func (g *ConfigManager) SetProxy(proxy string) {
	updateAppConfig(func(config *Config) {
		config.Proxy = proxy
	})
}

func (g *ConfigManager) SetSelected(email string) {
	updateAppConfig(func(config *Config) {
		config.Selected = email
	})
}

func (g *ConfigManager) SetUseQuota(useQuota bool) {
	updateAppConfig(func(config *Config) {
		config.UseQuota = useQuota
	})
}

func (g *ConfigManager) SetSaver(saver bool) {
	updateAppConfig(func(config *Config) {
		config.Saver = saver
	})
}

func (g *ConfigManager) SetRecursive(recursive bool) {
	updateAppConfig(func(config *Config) {
		config.Recursive = recursive
	})
}

func (g *ConfigManager) SetForceUpload(forceUpload bool) {
	updateAppConfig(func(config *Config) {
		config.ForceUpload = forceUpload
	})
}

func (g *ConfigManager) SetRecordUploads(enabled bool) error {
	ensureConfigLoaded()
	configMu.Lock()
	defer configMu.Unlock()
	previous := AppConfig.RecordUploads
	AppConfig.RecordUploads = enabled
	if err := saveAppConfigLocked(); err != nil {
		AppConfig.RecordUploads = previous
		return err
	}
	return nil
}

func (g *ConfigManager) SetSkipRecordedUploads(enabled bool) error {
	ensureConfigLoaded()
	configMu.Lock()
	defer configMu.Unlock()
	previous := AppConfig.SkipRecordedUploads
	AppConfig.SkipRecordedUploads = enabled
	if err := saveAppConfigLocked(); err != nil {
		AppConfig.SkipRecordedUploads = previous
		return err
	}
	return nil
}

func (g *ConfigManager) SetPairLivePhotos(pairLivePhotos bool) {
	updateAppConfig(func(config *Config) {
		config.PairLivePhotos = pairLivePhotos
	})
}

func (g *ConfigManager) SetSkipIncompleteLivePhotos(skipIncompleteLivePhotos bool) {
	updateAppConfig(func(config *Config) {
		config.SkipIncompleteLivePhotos = skipIncompleteLivePhotos
	})
}

func (g *ConfigManager) SetUpdateExistingPhotosToLive(updateExistingPhotosToLive bool) {
	updateAppConfig(func(config *Config) {
		config.UpdateExistingPhotosToLive = updateExistingPhotosToLive
	})
}

func (g *ConfigManager) SetDeleteFromHost(deleteFromHost bool) {
	updateAppConfig(func(config *Config) {
		config.DeleteFromHost = deleteFromHost
	})
}

func (g *ConfigManager) SetDisableUnsupportedFilesFilter(disableUnsupportedFilesFilter bool) {
	updateAppConfig(func(config *Config) {
		config.DisableUnsupportedFilesFilter = disableUnsupportedFilesFilter
	})
}

func (g *ConfigManager) SetUploadThreads(uploadThreads int) {
	if uploadThreads < 1 {
		return
	}
	updateAppConfig(func(config *Config) {
		config.UploadThreads = uploadThreads
	})
}

func (g *ConfigManager) SetAlbumName(albumName string) {
	configMu.Lock()
	defer configMu.Unlock()
	AppConfig.AlbumName = strings.TrimSpace(albumName)
}

func (g *ConfigManager) GetAlbumName() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.AlbumName
}

func (g *ConfigManager) SetAlbumAutoMode(autoMode bool) {
	configMu.Lock()
	defer configMu.Unlock()
	AppConfig.AlbumAutoMode = autoMode
	// Don't persist to disk - this is per-session like AlbumName
}

func (g *ConfigManager) GetAlbumAutoMode() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.AlbumAutoMode
}

// GetRecentAlbums returns the active account's recent album shortcuts. A copy
// is returned so callers cannot mutate configuration without persistence.
func (g *ConfigManager) GetRecentAlbums() []RecentAlbum {
	ensureConfigLoaded()
	configMu.RLock()
	defer configMu.RUnlock()

	items := AppConfig.RecentAlbumsByAccount[AppConfig.Selected]
	result := make([]RecentAlbum, len(items))
	copy(result, items)
	return result
}

// TouchRecentAlbum records a successfully updated, explicitly selected album.
// Auto Album folders are deliberately excluded by the caller to keep this
// shortcut list useful instead of filling it with one-off folder albums.
func (g *ConfigManager) TouchRecentAlbum(albumID, albumName string) {
	albumID = strings.TrimSpace(albumID)
	if albumID == "" {
		return
	}

	updateAppConfig(func(config *Config) {
		account := config.Selected
		if account == "" {
			return
		}
		if config.RecentAlbumsByAccount == nil {
			config.RecentAlbumsByAccount = make(map[string][]RecentAlbum)
		}

		items := config.RecentAlbumsByAccount[account]
		filtered := make([]RecentAlbum, 0, len(items)+1)
		for _, item := range items {
			if item.AlbumID != albumID {
				filtered = append(filtered, item)
			}
		}
		filtered = append(filtered, RecentAlbum{
			AlbumID:    albumID,
			AlbumName:  strings.TrimSpace(albumName),
			LastUsedAt: time.Now().UnixNano(),
		})
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].LastUsedAt > filtered[j].LastUsedAt
		})
		if len(filtered) > maxRecentAlbumsPerAccount {
			filtered = filtered[:maxRecentAlbumsPerAccount]
		}
		config.RecentAlbumsByAccount[account] = filtered
	})
}

// RemoveRecentAlbum forgets a shortcut when its album key is no longer valid.
func (g *ConfigManager) RemoveRecentAlbum(albumID string) {
	albumID = strings.TrimSpace(albumID)
	if albumID == "" {
		return
	}

	updateAppConfig(func(config *Config) {
		items := config.RecentAlbumsByAccount[config.Selected]
		filtered := make([]RecentAlbum, 0, len(items))
		for _, item := range items {
			if item.AlbumID != albumID {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) == 0 {
			delete(config.RecentAlbumsByAccount, config.Selected)
			return
		}
		config.RecentAlbumsByAccount[config.Selected] = filtered
	})
}

func (g *ConfigManager) GetAlbumHistory() []AlbumHistoryEntry {
	ensureConfigLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	items := AppConfig.AlbumHistoryByAccount[AppConfig.Selected]
	result := make([]AlbumHistoryEntry, len(items))
	copy(result, items)
	return result
}

func (g *ConfigManager) AddAlbumHistory(albumID, albumName string, itemsAdded int, operation string) {
	if strings.TrimSpace(albumID) == "" || itemsAdded <= 0 {
		return
	}
	updateAppConfig(func(config *Config) {
		if config.Selected == "" {
			return
		}
		if config.AlbumHistoryByAccount == nil {
			config.AlbumHistoryByAccount = make(map[string][]AlbumHistoryEntry)
		}
		name := strings.TrimSpace(albumName)
		if IsAlbumKey(name) || strings.HasPrefix(name, "Album (AF1Qip") {
			name = "Saved album"
		}
		items := append([]AlbumHistoryEntry{{AlbumID: albumID, AlbumName: name, ItemsAdded: itemsAdded, Operation: operation, CreatedAt: time.Now().Unix()}}, config.AlbumHistoryByAccount[config.Selected]...)
		if len(items) > maxAlbumHistoryPerAccount {
			items = items[:maxAlbumHistoryPerAccount]
		}
		config.AlbumHistoryByAccount[config.Selected] = items
	})
}

// ExportDebugLog writes a user-approved diagnostic report. Its contents are
// prepared by the frontend without credentials or authentication tokens.
func (g *ConfigManager) ExportDebugLog(path, contents string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("debug log path cannot be empty")
	}
	return os.WriteFile(path, []byte(contents), 0o600)
}

// GetAlbumConfig returns album name and auto mode atomically
func GetAlbumConfig() (albumName string, autoMode bool) {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.AlbumName, AppConfig.AlbumAutoMode
}

func (g *ConfigManager) SetSetDateFromFilename(v bool) {
	updateAppConfig(func(config *Config) {
		config.SetDateFromFilename = v
	})
}

func (g *ConfigManager) SetExcludePattern(pattern string) {
	updateAppConfig(func(config *Config) {
		config.ExcludePattern = pattern
	})
}

func (g *ConfigManager) GetExcludePattern() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.ExcludePattern
}

func (g *ConfigManager) AddCredentials(newAuthString string) error {
	// Required fields that must be present in the auth string
	requiredFields := []string{
		"androidId",
		"app",
		"client_sig",
		"Email",
		"Token",
		"lang",
		"service",
	}

	// Parse the auth string
	params, err := url.ParseQuery(newAuthString)
	if err != nil {
		return fmt.Errorf("invalid auth string format: %v", err)
	}

	// Validate required fields
	var missingFields []string
	for _, field := range requiredFields {
		if params.Get(field) == "" {
			missingFields = append(missingFields, field)
		}
	}
	if len(missingFields) > 0 {
		return fmt.Errorf("auth string missing required fields: %v", missingFields)
	}

	// Get and validate email
	email := params.Get("Email")
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	configMu.Lock()
	defer configMu.Unlock()

	// Check for duplicate email in existing credentials
	for _, cred := range AppConfig.Credentials {
		existingParams, err := url.ParseQuery(cred)
		if err != nil {
			continue // skip malformed entries
		}
		if existingParams.Get("Email") == email {
			return fmt.Errorf("auth string with email %s already exists", email)
		}
	}

	// If validation passed, add the new credentials
	AppConfig.Credentials = append(AppConfig.Credentials, newAuthString)
	AppConfig.Selected = email
	_ = saveAppConfigLocked()
	return nil
}

func (g *ConfigManager) CredentialNeedsTokenBinding(authString string) bool {
	params, err := url.ParseQuery(authString)
	if err != nil {
		return false
	}
	return credentialNeedsTokenBinding(params)
}

func (g *ConfigManager) AddTokenBindingAliasFromADB(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	alias, err := extractTokenBindingAliasFromADB(email)
	if err != nil {
		return err
	}

	configMu.Lock()
	defer configMu.Unlock()

	for i, cred := range AppConfig.Credentials {
		params, err := url.ParseQuery(cred)
		if err != nil {
			continue
		}
		if params.Get("Email") != email {
			continue
		}
		params.Set("token_binding_alias", alias)
		AppConfig.Credentials[i] = params.Encode()
		return saveAppConfigLocked()
	}

	return fmt.Errorf("no credentials found for email %s", email)
}

func (g *ConfigManager) RemoveCredentials(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	configMu.Lock()
	defer configMu.Unlock()

	// Find and remove the credential with matching email
	found := false
	var updatedCredentials []string

	for _, cred := range AppConfig.Credentials {
		params, err := url.ParseQuery(cred)
		if err != nil {
			continue // skip malformed entries
		}

		if params.Get("Email") == email {
			found = true
			continue // skip this credential (effectively removing it)
		}

		updatedCredentials = append(updatedCredentials, cred)
	}

	if !found {
		return fmt.Errorf("no credentials found for email %s", email)
	}

	// Update the configuration
	AppConfig.Credentials = updatedCredentials

	// If we're removing the currently selected credential, clear the selection
	if AppConfig.Selected == email {
		AppConfig.Selected = ""
	}

	_ = saveAppConfigLocked()
	return nil
}

func credentialNeedsTokenBinding(params url.Values) bool {
	if params.Get("token_binding_alias") != "" {
		return false
	}
	return params.Get("assertion_jwt") != "" ||
		params.Get("check_tb_upgrade_eligible") != ""
}

// accountsCEDBPath is the credential-encrypted AccountManager database for the
// primary (user 0) Android profile.
const accountsCEDBPath = "/data/system_ce/0/accounts_ce.db"

// errADBRootUnavailable signals that the device could not be read because root
// access was denied, as opposed to the database simply not containing the key.
var errADBRootUnavailable = errors.New("root access unavailable")

func extractTokenBindingAliasFromADB(email string) (string, error) {
	if _, err := exec.LookPath("adb"); err != nil {
		return "", fmt.Errorf("adb was not found in PATH")
	}

	escapedEmail := strings.ReplaceAll(email, "'", "''")
	query := fmt.Sprintf(
		"select extras.value from extras join accounts on accounts._id=extras.accounts_id where accounts.name='%s' and extras.key='lstBindingKeyAlias';",
		escapedEmail,
	)

	devices, err := listADBDevices()
	if err != nil {
		return "", err
	}

	var failures []string
	var reachableRoot bool
	for _, device := range devices {
		_ = exec.Command("adb", "-s", device, "root").Run()

		alias, rooted, err := readTokenBindingAliasFromDevice(device, query)
		if rooted {
			reachableRoot = true
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", device, cleanADBError(err.Error())))
			continue
		}

		alias = strings.TrimSpace(alias)
		if alias == "" {
			failures = append(failures, fmt.Sprintf("%s: no token binding key for %s", device, email))
			continue
		}
		if !strings.HasPrefix(alias, tokenBindingECDSAAliasPrefix) {
			failures = append(failures, fmt.Sprintf("%s: unsupported token binding key format", device))
			continue
		}

		return alias, nil
	}

	if reachableRoot {
		return "", fmt.Errorf("token binding alias not found for %s on any connected adb device (%s)", email, strings.Join(failures, "; "))
	}
	return "", fmt.Errorf("could not read Android AccountManager on any connected adb device; root is required (%s)", strings.Join(failures, "; "))
}

// readTokenBindingAliasFromDevice pulls the AccountManager database from a single
// device and runs the lookup against the local copy. The pulled database is
// sensitive (it holds auth material for every account on the device), so the
// temporary copy is always removed via defer — including on a panic. The rooted
// return reports whether the device was reachable with root, used to produce an
// accurate error message.
func readTokenBindingAliasFromDevice(device, query string) (alias string, rooted bool, err error) {
	dbPath, cleanup, err := pullAccountsDB(device)
	if err != nil {
		// A non-root failure means we did reach the device with root but
		// something else went wrong (e.g. the db file is missing).
		return "", !errors.Is(err, errADBRootUnavailable), err
	}
	defer cleanup()

	alias, err = queryTokenBindingAlias(dbPath, query)
	if err != nil {
		return "", true, err
	}
	return alias, true, nil
}

func listADBDevices() ([]string, error) {
	out, err := exec.Command("adb", "devices").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list adb devices: %s", cleanADBError(string(out)))
	}

	var devices []string
	var unavailable []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "List" {
			continue
		}
		switch fields[1] {
		case "device":
			devices = append(devices, fields[0])
		case "offline", "unauthorized", "no permissions":
			unavailable = append(unavailable, fmt.Sprintf("%s is %s", fields[0], fields[1]))
		default:
			unavailable = append(unavailable, fmt.Sprintf("%s is %s", fields[0], fields[1]))
		}
	}

	if len(devices) == 0 {
		if len(unavailable) > 0 {
			return nil, fmt.Errorf("no usable adb devices found (%s)", strings.Join(unavailable, "; "))
		}
		return nil, fmt.Errorf("no adb devices found")
	}

	return devices, nil
}

// pullAccountsDB streams the AccountManager database off the device via root and
// writes it to a local temporary directory. Modern Android no longer ships the
// sqlite3 binary, so the database is queried locally instead of on-device. The
// returned cleanup function removes the temporary files.
func pullAccountsDB(device string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "gotohp-adb-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	mainDest := filepath.Join(tmpDir, "accounts_ce.db")
	if err := streamDeviceFile(device, accountsCEDBPath, mainDest); err != nil {
		cleanup()
		return "", nil, err
	}

	// accounts_ce.db is typically in WAL mode; copy the companion files so the
	// local query observes the latest writes. They may not exist, so ignore
	// failures here.
	_ = streamDeviceFile(device, accountsCEDBPath+"-wal", mainDest+"-wal")
	_ = streamDeviceFile(device, accountsCEDBPath+"-shm", mainDest+"-shm")

	return mainDest, cleanup, nil
}

// streamDeviceFile copies a single root-owned file off the device to localPath.
// adb exec-out is used (rather than adb shell) to avoid the CRLF translation
// that would corrupt the binary database.
func streamDeviceFile(device, remotePath, localPath string) error {
	cmd := exec.Command("adb", "-s", device, "exec-out", "su", "-c", fmt.Sprintf("cat %q", remotePath))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	// Some su builds exit 0 even when cat fails, so also treat stderr-with-no-
	// output as a failure.
	if runErr != nil || (stderr.Len() > 0 && stdout.Len() == 0) {
		msg := cleanADBError(stderr.String())
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		if isADBRootFailure(msg) {
			return fmt.Errorf("%w: %s", errADBRootUnavailable, msg)
		}
		return fmt.Errorf("failed to read %s: %s", remotePath, msg)
	}

	if err := os.WriteFile(localPath, stdout.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write local copy: %w", err)
	}
	return nil
}

// isADBRootFailure reports whether a device error indicates missing/denied root
// rather than a missing file (which means root worked but the data is absent).
func isADBRootFailure(msg string) bool {
	m := strings.ToLower(msg)
	if strings.Contains(m, "no such file") {
		return false
	}
	return strings.Contains(m, "su:") ||
		strings.Contains(m, "permission denied") ||
		strings.Contains(m, "not allowed") ||
		strings.Contains(m, "inaccessible or not found")
}

// queryTokenBindingAlias opens the pulled database locally with a pure-Go SQLite
// driver and runs the lookup query. The local copy is private to this process,
// so it is opened read-write to allow WAL recovery.
func queryTokenBindingAlias(dbPath, query string) (string, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open accounts db: %w", err)
	}
	defer func() { _ = db.Close() }()

	var alias string
	if err := db.QueryRow(query).Scan(&alias); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to query token binding alias: %w", err)
	}
	return alias, nil
}

func cleanADBError(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return "adb command failed"
	}
	return strings.Join(strings.Fields(out), " ")
}

func determineConfigPath() {
	// First try portable config in executable directory
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		portableConfigPath := filepath.Join(exeDir, "gotohp.config")

		// If config exists in executable directory, use it
		if _, err := os.Stat(portableConfigPath); err == nil {
			ConfigPath = portableConfigPath
			return
		}
	}

	// Fall back to default location
	userConfigDir := filepath.Join(getUserConfigDir(), "/gotohp")
	ConfigPath = filepath.Join(userConfigDir, "gotohp.config")
}

func getUserConfigDir() string {
	dirname, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	return dirname
}

//wails:ignore
func (g *ConfigManager) GetConfig() Config {
	ensureConfigLoaded()
	configMu.RLock()
	defer configMu.RUnlock()

	return AppConfig
}

func (g *ConfigManager) GetSettings() Config {
	ensureConfigLoaded()
	configMu.RLock()
	defer configMu.RUnlock()

	settings := AppConfig
	settings.Credentials = nil
	return settings
}

func (g *ConfigManager) GetAccounts() AccountsState {
	ensureConfigLoaded()
	configMu.RLock()
	defer configMu.RUnlock()

	state := AccountsState{
		Accounts: make([]AccountSummary, 0, len(AppConfig.Credentials)),
		Selected: AppConfig.Selected,
	}
	for _, credential := range AppConfig.Credentials {
		values, err := url.ParseQuery(credential)
		if err != nil || values.Get("Email") == "" {
			continue
		}
		state.Accounts = append(state.Accounts, AccountSummary{
			Email:             values.Get("Email"),
			NeedsTokenBinding: credentialNeedsTokenBinding(values),
		})
	}
	return state
}

func ensureConfigLoaded() {
	configMu.RLock()
	loaded := ConfigPath != ""
	configMu.RUnlock()
	if loaded {
		return
	}

	configMu.Lock()
	defer configMu.Unlock()
	if ConfigPath == "" {
		_ = loadConfigLocked()
	}
}

// LoadConfig loads the configuration (exported for CLI use)
func LoadConfig() error {
	configMu.Lock()
	defer configMu.Unlock()
	return loadConfigLocked()
}

func loadConfigLocked() error {
	determineConfigPath()

	file, _ := os.ReadFile(ConfigPath)
	if len(file) == 0 {
		AppConfig = DefaultConfig
	} else {
		AppConfig = loadAppConfig()
		_ = os.Chmod(ConfigPath, 0o600)
	}

	return nil
}

func updateAppConfig(update func(*Config)) {
	configMu.Lock()
	defer configMu.Unlock()

	update(&AppConfig)
	_ = saveAppConfigLocked()
}

// saveAppConfigLocked persists AppConfig while the caller holds configMu for writing.
func saveAppConfigLocked() error {
	k := koanf.New(".")

	err := k.Load(structs.Provider(AppConfig, "koanf"), nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ConfigPath), 0o700); err != nil {
		return err
	}
	b, err := k.Marshal(yaml.Parser())
	if err != nil {
		fmt.Println(err)
		return err
	}

	return writeConfigAtomically(ConfigPath, b)
}

func writeConfigAtomically(path string, contents []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".gotohp-config-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func loadAppConfig() Config {
	c := DefaultConfig
	k := koanf.New(".")
	if err := k.Load(file.Provider(ConfigPath), yaml.Parser()); err != nil {
		log.Printf("error parsing app config: %v", err)
		return DefaultConfig
	}
	err := k.Unmarshal("", &c)
	if err != nil {
		log.Printf("error unmarshaling app config: %v", err)
		return DefaultConfig
	}

	if !k.Exists("skip_incomplete_live_photos") {
		c.SkipIncompleteLivePhotos = DefaultConfig.SkipIncompleteLivePhotos
	}

	if c.UploadThreads < 1 {
		c.UploadThreads = DefaultConfig.UploadThreads
	}

	return c
}
