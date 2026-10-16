package backend

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupAvatarConfig(t *testing.T, accounts ...string) {
	t.Helper()
	configMu.Lock()
	previousConfig, previousPath := AppConfig, ConfigPath
	credentials := make([]string, len(accounts))
	for i, account := range accounts {
		credentials[i] = "Email=" + account
	}
	AppConfig = Config{Credentials: credentials}
	ConfigPath = filepath.Join(t.TempDir(), "gotohp.config")
	configMu.Unlock()
	t.Cleanup(func() {
		configMu.Lock()
		AppConfig, ConfigPath = previousConfig, previousPath
		configMu.Unlock()
	})
}

func writeTestAvatar(t *testing.T, path string, shade uint8) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: shade, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAccountAvatarsAreLocalAndScopedToEachAccount(t *testing.T) {
	setupAvatarConfig(t, "one@example.com", "two@example.com")
	sourceDir := t.TempDir()
	one, two := filepath.Join(sourceDir, "one.png"), filepath.Join(sourceDir, "two.png")
	writeTestAvatar(t, one, 10)
	writeTestAvatar(t, two, 20)

	manager := &ConfigManager{}
	if err := manager.SetAccountAvatar("one@example.com", one); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetAccountAvatar("two@example.com", two); err != nil {
		t.Fatal(err)
	}

	accounts := manager.GetAccounts().Accounts
	if len(accounts) != 2 || !strings.HasPrefix(accounts[0].AvatarDataURL, "data:image/png;base64,") || !strings.HasPrefix(accounts[1].AvatarDataURL, "data:image/png;base64,") {
		t.Fatalf("account avatars not returned: %+v", accounts)
	}
	if accounts[0].AvatarDataURL == accounts[1].AvatarDataURL {
		t.Fatal("accounts unexpectedly share the same avatar")
	}
	if _, err := os.Stat(avatarDirectory()); err != nil {
		t.Fatalf("avatar was not copied into app storage: %v", err)
	}
}

func TestSetAccountAvatarRejectsInvalidAndOversizedFilesWithoutReplacingCurrentAvatar(t *testing.T) {
	setupAvatarConfig(t, "one@example.com")
	manager := &ConfigManager{}
	sourceDir := t.TempDir()
	valid := filepath.Join(sourceDir, "avatar.png")
	writeTestAvatar(t, valid, 10)
	if err := manager.SetAccountAvatar("one@example.com", valid); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(sourceDir, "replacement.png")
	writeTestAvatar(t, replacement, 30)
	if err := manager.SetAccountAvatar("one@example.com", replacement); err != nil {
		t.Fatalf("replacing an existing avatar failed: %v", err)
	}
	before := manager.GetAccounts().Accounts[0].AvatarDataURL

	invalid := filepath.Join(sourceDir, "not-image.png")
	if err := os.WriteFile(invalid, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetAccountAvatar("one@example.com", invalid); err == nil {
		t.Fatal("invalid image was accepted")
	}

	oversized := filepath.Join(sourceDir, "large.png")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte{0}, maxAccountAvatarBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetAccountAvatar("one@example.com", oversized); err == nil {
		t.Fatal("oversized image was accepted")
	}
	if got := manager.GetAccounts().Accounts[0].AvatarDataURL; got != before {
		t.Fatal("failed replacement changed the saved avatar")
	}
	if _, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(before, "data:image/png;base64,")); err != nil {
		t.Fatalf("saved avatar is not valid base64: %v", err)
	}
}

func TestRemovingAccountDeletesItsAvatarOnly(t *testing.T) {
	setupAvatarConfig(t, "one@example.com", "two@example.com")
	sourceDir := t.TempDir()
	manager := &ConfigManager{}
	for i, account := range []string{"one@example.com", "two@example.com"} {
		path := filepath.Join(sourceDir, account+".png")
		writeTestAvatar(t, path, uint8(10+i))
		if err := manager.SetAccountAvatar(account, path); err != nil {
			t.Fatal(err)
		}
	}
	onePath := accountAvatarPath("one@example.com")
	twoPath := accountAvatarPath("two@example.com")

	if err := manager.RemoveCredentials("one@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(onePath); !os.IsNotExist(err) {
		t.Fatalf("removed account avatar still exists: %v", err)
	}
	if _, err := os.Stat(twoPath); err != nil {
		t.Fatalf("other account avatar was removed: %v", err)
	}
}

func TestClearAccountAvatarRemovesOnlyThatAccountImage(t *testing.T) {
	setupAvatarConfig(t, "one@example.com", "two@example.com")
	sourceDir := t.TempDir()
	manager := &ConfigManager{}
	for i, account := range []string{"one@example.com", "two@example.com"} {
		path := filepath.Join(sourceDir, account+".png")
		writeTestAvatar(t, path, uint8(10+i))
		if err := manager.SetAccountAvatar(account, path); err != nil {
			t.Fatal(err)
		}
	}

	if err := manager.ClearAccountAvatar("one@example.com"); err != nil {
		t.Fatal(err)
	}
	accounts := manager.GetAccounts().Accounts
	if accounts[0].AvatarDataURL != "" || accounts[1].AvatarDataURL == "" {
		t.Fatalf("clearing one avatar affected wrong account: %+v", accounts)
	}
}
