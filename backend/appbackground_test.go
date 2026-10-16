package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppBackgroundImageAndColorAreLocalAndResettable(t *testing.T) {
	setupAvatarConfig(t)
	manager := &ConfigManager{}
	source := filepath.Join(t.TempDir(), "background.png")
	writeTestAvatar(t, source, 80)
	if err := manager.SetAppBackgroundImage(source); err != nil {
		t.Fatal(err)
	}
	imageKey := AppConfig.BackgroundImageKey
	if AppConfig.BackgroundMode != "image" || imageKey == "" {
		t.Fatalf("image background was not selected: %+v", AppConfig)
	}
	if got := manager.GetAppBackgroundImageDataURL(); !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("image data URL was not returned: %q", got)
	}
	if _, err := os.Stat(backgroundImagePath(imageKey)); err != nil {
		t.Fatalf("image was not copied to local app storage: %v", err)
	}
	if err := manager.SetAppBackgroundColor("#aabbcc"); err != nil {
		t.Fatal(err)
	}
	if AppConfig.BackgroundMode != "color" || AppConfig.BackgroundColor != "#AABBCC" || AppConfig.BackgroundImageKey != imageKey {
		t.Fatalf("color selection did not preserve saved image: %+v", AppConfig)
	}
	if err := manager.ResetAppBackground(); err != nil {
		t.Fatal(err)
	}
	if AppConfig.BackgroundMode != "default" || AppConfig.BackgroundColor != "" || AppConfig.BackgroundImageKey != "" {
		t.Fatalf("background was not reset: %+v", AppConfig)
	}
	if _, err := os.Stat(backgroundImagePath(imageKey)); !os.IsNotExist(err) {
		t.Fatalf("reset did not remove custom local image: %v", err)
	}
}

func TestSetAppBackgroundRejectsInvalidColorAndImage(t *testing.T) {
	setupAvatarConfig(t)
	manager := &ConfigManager{}
	if err := manager.SetAppBackgroundColor("not-a-color"); err == nil {
		t.Fatal("invalid color was accepted")
	}
	invalid := filepath.Join(t.TempDir(), "background.png")
	if err := os.WriteFile(invalid, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetAppBackgroundImage(invalid); err == nil {
		t.Fatal("invalid image was accepted")
	}
	if AppConfig.BackgroundMode != "" || AppConfig.BackgroundImageKey != "" {
		t.Fatalf("invalid input changed config: %+v", AppConfig)
	}
}

func TestResetAppBackgroundDoesNotDeleteOutsideItsImageDirectory(t *testing.T) {
	setupAvatarConfig(t)
	manager := &ConfigManager{}
	sentinel := filepath.Join(filepath.Dir(ConfigPath), "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	AppConfig.BackgroundImageKey = "../keep.txt"
	if err := manager.ResetAppBackground(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("reset removed a file outside background storage: %v", err)
	}
}

func TestAppBackgroundIsScopedToSelectedAccount(t *testing.T) {
	setupAvatarConfig(t, "one@example.com", "two@example.com")
	manager := &ConfigManager{}
	manager.SetSelected("one@example.com")
	if err := manager.SetAppBackgroundColor("#aabbcc"); err != nil {
		t.Fatal(err)
	}
	manager.SetSelected("two@example.com")
	if got := manager.GetSettings().BackgroundMode; got != "" {
		t.Fatalf("second account inherited the first account's background mode: %q", got)
	}
	if err := manager.SetAppBackgroundColor("#112233"); err != nil {
		t.Fatal(err)
	}
	manager.SetSelected("one@example.com")
	settings := manager.GetSettings()
	if settings.BackgroundMode != "color" || settings.BackgroundColor != "#AABBCC" {
		t.Fatalf("first account background changed after switching accounts: %+v", settings)
	}
}

func TestAppBackgroundImageRemainsPrivateToItsAccount(t *testing.T) {
	setupAvatarConfig(t, "one@example.com", "two@example.com")
	manager := &ConfigManager{}
	source := filepath.Join(t.TempDir(), "background.png")
	writeTestAvatar(t, source, 60)
	manager.SetSelected("one@example.com")
	if err := manager.SetAppBackgroundImage(source); err != nil {
		t.Fatal(err)
	}
	key := AppConfig.BackgroundsByAccount[backgroundAccountKey("one@example.com")].ImageKey
	manager.SetSelected("two@example.com")
	if got := manager.GetAppBackgroundImageDataURL(); got != "" {
		t.Fatal("second account received the first account's image")
	}
	if err := manager.ResetAppBackground(); err != nil {
		t.Fatal(err)
	}
	manager.SetSelected("one@example.com")
	if got := manager.GetAppBackgroundImageDataURL(); !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("resetting second account removed first account's image: %q", got)
	}
	if err := manager.ResetAppBackground(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backgroundImagePath(key)); !os.IsNotExist(err) {
		t.Fatalf("reset did not remove unused image: %v", err)
	}
}
