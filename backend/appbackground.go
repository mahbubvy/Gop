package backend

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (g *ConfigManager) SetAppBackgroundColor(value string) error {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return fmt.Errorf("choose a valid color")
	}
	if _, err := hex.DecodeString(value[1:]); err != nil {
		return fmt.Errorf("choose a valid color")
	}

	ensureConfigLoaded()
	configMu.Lock()
	defer configMu.Unlock()
	background := selectedBackgroundLocked(AppConfig.Selected)
	background.Mode = "color"
	background.Color = strings.ToUpper(value)
	return setSelectedBackgroundLocked(background)
}

func (g *ConfigManager) SetAppBackgroundImage(sourcePath string) error {
	ensureConfigLoaded()
	source, err := os.Open(strings.TrimSpace(sourcePath))
	if err != nil {
		return fmt.Errorf("could not read selected image")
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxAccountAvatarBytes {
		return fmt.Errorf("background image must be a regular PNG, JPEG, or GIF under 5 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(source, maxAccountAvatarBytes+1))
	if err != nil || len(data) > maxAccountAvatarBytes {
		return fmt.Errorf("background image exceeds 5 MiB or could not be read")
	}
	_, err = validateAccountAvatar(data)
	if err != nil {
		return fmt.Errorf("background image: %w", err)
	}

	configMu.Lock()
	defer configMu.Unlock()
	if selected := strings.TrimSpace(AppConfig.Selected); selected != "" && !hasAccount(AppConfig.Credentials, selected) {
		return fmt.Errorf("selected account was not found")
	}
	previous := selectedBackgroundLocked(AppConfig.Selected)
	background := previous
	background.Mode = "image"
	background.ImageKey = backgroundImageKey(data)
	if err := writeBackgroundImage(background.ImageKey, data); err != nil {
		return err
	}
	if err := setSelectedBackgroundLocked(background); err != nil {
		removeUnusedBackgroundImageLocked(background.ImageKey)
		return err
	}
	if previous.ImageKey != background.ImageKey {
		removeUnusedBackgroundImageLocked(previous.ImageKey)
	}
	return nil
}

func (g *ConfigManager) ResetAppBackground() error {
	ensureConfigLoaded()
	configMu.Lock()
	defer configMu.Unlock()
	previous := selectedBackgroundLocked(AppConfig.Selected)
	if err := setSelectedBackgroundLocked(AppBackground{Mode: "default"}); err != nil {
		return err
	}
	removeUnusedBackgroundImageLocked(previous.ImageKey)
	return nil
}

func (g *ConfigManager) GetAppBackgroundImageDataURL() string {
	ensureConfigLoaded()
	configMu.RLock()
	key := selectedBackgroundLocked(AppConfig.Selected).ImageKey
	configMu.RUnlock()
	if len(key) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(key); err != nil {
		return ""
	}
	directory := backgroundDirectory()
	if err := validateAvatarDirectory(directory); err != nil {
		return ""
	}
	path := backgroundImagePath(key)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > maxAccountAvatarBytes {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	format, err := validateAccountAvatar(data)
	if err != nil {
		return ""
	}
	mimeType := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif"}[format]
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func selectedBackgroundLocked(email string) AppBackground {
	if key := backgroundAccountKey(email); key != "" {
		if background, ok := AppConfig.BackgroundsByAccount[key]; ok {
			return background
		}
	}
	return AppBackground{
		Mode:     AppConfig.BackgroundMode,
		Color:    AppConfig.BackgroundColor,
		ImageKey: AppConfig.BackgroundImageKey,
	}
}

func setSelectedBackgroundLocked(background AppBackground) error {
	email := strings.TrimSpace(AppConfig.Selected)
	previous := AppConfig
	if email == "" {
		AppConfig.BackgroundMode = background.Mode
		AppConfig.BackgroundColor = background.Color
		AppConfig.BackgroundImageKey = background.ImageKey
	} else {
		if !hasAccount(AppConfig.Credentials, email) {
			return fmt.Errorf("selected account was not found")
		}
		accountBackgrounds := make(map[string]AppBackground, len(AppConfig.BackgroundsByAccount)+1)
		for key, value := range AppConfig.BackgroundsByAccount {
			accountBackgrounds[key] = value
		}
		accountBackgrounds[backgroundAccountKey(email)] = background
		AppConfig.BackgroundsByAccount = accountBackgrounds
	}
	if err := saveAppConfigLocked(); err != nil {
		AppConfig = previous
		return fmt.Errorf("could not save background setting")
	}
	return nil
}

func backgroundAccountKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func removeUnusedBackgroundImageLocked(key string) {
	if key == "" || AppConfig.BackgroundImageKey == key {
		return
	}
	for _, background := range AppConfig.BackgroundsByAccount {
		if background.ImageKey == key {
			return
		}
	}
	_ = removeBackgroundImage(key)
}

func backgroundImageKey(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("%x", digest)
}

func backgroundDirectory() string {
	return filepath.Join(filepath.Dir(ConfigPath), "backgrounds")
}

func backgroundImagePath(key string) string {
	return filepath.Join(backgroundDirectory(), key)
}

func removeBackgroundImage(key string) error {
	if len(key) != 64 {
		return fmt.Errorf("invalid local background key")
	}
	if _, err := hex.DecodeString(key); err != nil {
		return fmt.Errorf("invalid local background key")
	}
	directory := backgroundDirectory()
	if err := validateAvatarDirectory(directory); err != nil {
		return err
	}
	path := backgroundImagePath(key)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("local background image is not safe to remove")
	}
	return os.Remove(path)
}

func writeBackgroundImage(key string, data []byte) error {
	directory := backgroundDirectory()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("could not create local background storage")
	}
	if err := validateAvatarDirectory(directory); err != nil {
		return err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("could not secure local background storage")
	}
	target := backgroundImagePath(key)
	if info, err := os.Lstat(target); err == nil {
		if info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Size() == int64(len(data)) {
			return nil
		}
		return fmt.Errorf("local background path is not a safe file")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not access local background storage")
	}
	temporary, err := os.CreateTemp(directory, ".background-*")
	if err != nil {
		return fmt.Errorf("could not save local background image")
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("could not save local background image")
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("could not save local background image")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("could not save local background image")
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("could not save local background image")
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return fmt.Errorf("could not save local background image")
	}
	return nil
}
