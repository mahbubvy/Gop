package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSavedAlbumsAreAccountScopedAndPersisted(t *testing.T) {
	configMu.Lock()
	previousConfig, previousPath := AppConfig, ConfigPath
	ConfigPath = filepath.Join(t.TempDir(), "gotohp.config")
	AppConfig = Config{Selected: "account-a"}
	configMu.Unlock()
	t.Cleanup(func() {
		configMu.Lock()
		AppConfig, ConfigPath = previousConfig, previousPath
		configMu.Unlock()
	})

	manager := &ConfigManager{}
	if err := manager.SaveSavedAlbum("AF1QipOne", "Family photos"); err != nil {
		t.Fatal(err)
	}
	if err := manager.SaveSavedAlbum("AF1QipOne", "Family renamed"); err != nil {
		t.Fatal(err)
	}
	if err := manager.SaveSavedAlbum("not-an-id", "Invalid"); err == nil {
		t.Fatal("accepted a non-album identifier")
	}
	if err := manager.SaveSavedAlbum("AF1QipTwo", "  "); err == nil {
		t.Fatal("accepted an empty display name")
	}

	configMu.Lock()
	AppConfig.Selected = "account-b"
	configMu.Unlock()
	if got := manager.GetSavedAlbums(); len(got) != 0 {
		t.Fatalf("account B saw account A's saved albums: %+v", got)
	}
	if err := manager.SaveSavedAlbum("AF1QipOne", "Account B name"); err != nil {
		t.Fatal(err)
	}

	configMu.Lock()
	AppConfig.Selected = "account-a"
	configMu.Unlock()
	got := manager.GetSavedAlbums()
	if len(got) != 1 || got[0].AlbumID != "AF1QipOne" || got[0].AlbumName != "Family renamed" {
		t.Fatalf("upsert failed or crossed accounts: %+v", got)
	}
	if contents, err := os.ReadFile(ConfigPath); err != nil || len(contents) == 0 {
		t.Fatalf("saved album config was not persisted: %v", err)
	}
	if persisted := loadAppConfig().SavedAlbumsByAccount["account-a"]; len(persisted) != 1 || persisted[0].AlbumName != "Family renamed" {
		t.Fatalf("saved album did not survive config reload: %+v", persisted)
	}

	if err := manager.RemoveSavedAlbum("AF1QipOne"); err != nil {
		t.Fatal(err)
	}
	if got := manager.GetSavedAlbums(); len(got) != 0 {
		t.Fatalf("remove left a saved album: %+v", got)
	}
	configMu.Lock()
	AppConfig.Selected = "account-b"
	configMu.Unlock()
	if got := manager.GetSavedAlbums(); len(got) != 1 || got[0].AlbumName != "Account B name" {
		t.Fatalf("remove affected another account: %+v", got)
	}
}

func TestSavedAlbumListDoesNotUseRecentAlbumLimit(t *testing.T) {
	configMu.Lock()
	previousConfig, previousPath := AppConfig, ConfigPath
	ConfigPath = filepath.Join(t.TempDir(), "gotohp.config")
	AppConfig = Config{Selected: "account-a"}
	configMu.Unlock()
	t.Cleanup(func() {
		configMu.Lock()
		AppConfig, ConfigPath = previousConfig, previousPath
		configMu.Unlock()
	})

	manager := &ConfigManager{}
	for i := range 15 {
		id := "AF1Qip" + string(rune('A'+i))
		if err := manager.SaveSavedAlbum(id, "Album "+id); err != nil {
			t.Fatal(err)
		}
		manager.TouchRecentAlbum(id, "Album "+id)
	}
	if got := manager.GetSavedAlbums(); len(got) != 15 {
		t.Fatalf("saved list was capped: got %d entries", len(got))
	}
	if got := manager.GetRecentAlbums(); len(got) != maxRecentAlbumsPerAccount {
		t.Fatalf("recent list limit changed: got %d entries", len(got))
	}
}
