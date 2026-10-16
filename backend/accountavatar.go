package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const maxAccountAvatarBytes = 5 << 20

// ponytail: 20MP bounds decoder memory; raise only if real avatar files require it.
const maxAccountAvatarPixels = 20_000_000

func (g *ConfigManager) SetAccountAvatar(email, sourcePath string) error {
	ensureConfigLoaded()
	email = strings.TrimSpace(email)
	if email == "" || strings.TrimSpace(sourcePath) == "" {
		return fmt.Errorf("account and image file are required")
	}
	sourcePath = strings.TrimSpace(sourcePath)

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("could not read selected image")
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("selected image is not a regular file")
	}
	if info.Size() <= 0 || info.Size() > maxAccountAvatarBytes {
		return fmt.Errorf("avatar image must be between 1 byte and 5 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(source, maxAccountAvatarBytes+1))
	if err != nil || len(data) > maxAccountAvatarBytes {
		return fmt.Errorf("could not read selected image or image exceeds 5 MiB")
	}
	if _, err := validateAccountAvatar(data); err != nil {
		return err
	}

	configMu.Lock()
	defer configMu.Unlock()
	if !hasAccount(AppConfig.Credentials, email) {
		return fmt.Errorf("no credentials found for this account")
	}
	return writeAccountAvatar(email, data)
}

func (g *ConfigManager) ClearAccountAvatar(email string) error {
	ensureConfigLoaded()
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("account is required")
	}

	configMu.Lock()
	defer configMu.Unlock()
	if !hasAccount(AppConfig.Credentials, email) {
		return fmt.Errorf("no credentials found for this account")
	}
	return removeAccountAvatar(email)
}

func validateAccountAvatar(data []byte) (string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !supportedAvatarFormat(format) {
		return "", fmt.Errorf("choose a PNG, JPEG, or GIF image")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxAccountAvatarPixels {
		return "", fmt.Errorf("avatar dimensions are too large")
	}
	_, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format {
		return "", fmt.Errorf("image file is invalid")
	}
	return format, nil
}

func supportedAvatarFormat(format string) bool {
	return format == "png" || format == "jpeg" || format == "gif"
}

func avatarDirectory() string {
	return filepath.Join(filepath.Dir(ConfigPath), "avatars")
}

func accountAvatarPath(email string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return filepath.Join(avatarDirectory(), hex.EncodeToString(digest[:]))
}

func writeAccountAvatar(email string, data []byte) error {
	directory := avatarDirectory()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("could not create local avatar storage")
	}
	if err := validateAvatarDirectory(directory); err != nil {
		return err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("could not secure local avatar storage")
	}

	target := accountAvatarPath(email)
	if info, err := os.Lstat(target); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("local avatar path is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not access local avatar storage")
	}
	temporary, err := os.CreateTemp(directory, ".avatar-*")
	if err != nil {
		return fmt.Errorf("could not save local avatar")
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("could not save local avatar")
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("could not save local avatar")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("could not save local avatar")
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("could not save local avatar")
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return fmt.Errorf("could not save local avatar")
	}
	return nil
}

func validateAvatarDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("local avatar storage is not a safe directory")
	}
	return nil
}

func accountAvatarDataURL(email string) string {
	directory := avatarDirectory()
	if validateAvatarDirectory(directory) != nil {
		return ""
	}
	path := accountAvatarPath(email)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxAccountAvatarBytes {
		return ""
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return ""
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative != filepath.Base(path) {
		return ""
	}
	data, err := os.ReadFile(target)
	if err != nil || len(data) == 0 || len(data) > maxAccountAvatarBytes {
		return ""
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !supportedAvatarFormat(format) {
		return ""
	}
	mimeType := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif"}[format]
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func removeAccountAvatar(email string) error {
	directory := avatarDirectory()
	if _, err := os.Lstat(directory); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("could not access local avatar storage")
	}
	if err := validateAvatarDirectory(directory); err != nil {
		return err
	}
	path := accountAvatarPath(email)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("local avatar file is not safe to remove")
	}
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return fmt.Errorf("could not verify local avatar storage")
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("could not verify local avatar file")
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative != filepath.Base(path) || filepath.IsAbs(relative) {
		return fmt.Errorf("local avatar file is outside app storage")
	}
	return os.Remove(target)
}

func hasAccount(credentials []string, email string) bool {
	for _, credential := range credentials {
		values, err := url.ParseQuery(credential)
		if err == nil && strings.EqualFold(values.Get("Email"), email) {
			return true
		}
	}
	return false
}
