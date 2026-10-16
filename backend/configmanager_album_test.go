package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameAlbumHistoryEntries(t *testing.T) {
	entries := []AlbumHistoryEntry{
		{AlbumID: "AF1QipOne", AlbumName: "Saved album"},
		{AlbumID: "AF1QipTwo", AlbumName: "Other album"},
		{AlbumID: "AF1QipOne", AlbumName: "Older name"},
	}

	got := renameAlbumHistoryEntries(entries, "AF1QipOne", "Family photos")

	if got[0].AlbumName != "Family photos" || got[2].AlbumName != "Family photos" {
		t.Fatalf("matching entries were not renamed: %+v", got)
	}
	if got[1].AlbumName != "Other album" {
		t.Fatalf("unrelated album name changed: %+v", got[1])
	}
	if entries[0].AlbumName != "Saved album" {
		t.Fatal("input history was unexpectedly modified")
	}
}

func TestAddAlbumHistoryStoresFriendlyNameLocallyAndUpdatesMatchingEntries(t *testing.T) {
	configMu.Lock()
	previousConfig, previousPath := AppConfig, ConfigPath
	AppConfig = Config{
		Selected: "account-a",
		AlbumHistoryByAccount: map[string][]AlbumHistoryEntry{
			"account-a": {
				{AlbumID: "AF1QipOne", AlbumName: "Saved album", ItemsAdded: 2},
				{AlbumID: "AF1QipTwo", AlbumName: "Other album", ItemsAdded: 1},
			},
			"account-b": {{AlbumID: "AF1QipOne", AlbumName: "Another account", ItemsAdded: 1}},
		},
	}
	ConfigPath = filepath.Join(t.TempDir(), "gotohp.config")
	configMu.Unlock()
	t.Cleanup(func() {
		configMu.Lock()
		AppConfig, ConfigPath = previousConfig, previousPath
		configMu.Unlock()
	})

	(&ConfigManager{}).AddAlbumHistory("AF1QipOne", "Family photos", 3, "Updated")
	(&ConfigManager{}).TouchRecentAlbum("AF1QipOne", "Family photos")

	configMu.RLock()
	accountA := append([]AlbumHistoryEntry(nil), AppConfig.AlbumHistoryByAccount["account-a"]...)
	accountB := append([]AlbumHistoryEntry(nil), AppConfig.AlbumHistoryByAccount["account-b"]...)
	recent := append([]RecentAlbum(nil), AppConfig.RecentAlbumsByAccount["account-a"]...)
	configMu.RUnlock()
	if len(accountA) != 3 || accountA[0].AlbumName != "Family photos" || accountA[1].AlbumName != "Family photos" {
		t.Fatalf("new and prior local history entries should use the friendly name: %+v", accountA)
	}
	if accountA[2].AlbumName != "Other album" || accountB[0].AlbumName != "Another account" {
		t.Fatalf("renaming should affect only this album in this account: accountA=%+v accountB=%+v", accountA, accountB)
	}
	if len(recent) != 1 || recent[0].AlbumID != "AF1QipOne" || recent[0].AlbumName != "Family photos" {
		t.Fatalf("recent shortcut should store the local friendly name: %+v", recent)
	}

	contents, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "Family photos") {
		t.Fatal("friendly name was not saved to the local config file")
	}

	(&ConfigManager{}).AddAlbumHistory("AF1QipOne", "Unsuccessful rename", 0, "Updated")
	configMu.RLock()
	unchangedHistory := append([]AlbumHistoryEntry(nil), AppConfig.AlbumHistoryByAccount["account-a"]...)
	configMu.RUnlock()
	if len(unchangedHistory) != len(accountA) || unchangedHistory[0].AlbumName != "Family photos" {
		t.Fatalf("an operation with no added files should not change the local name: %+v", unchangedHistory)
	}
}
