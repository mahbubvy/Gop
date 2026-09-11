package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewAutoAlbumsUsesContainingFolders(t *testing.T) {
	dir := t.TempDir()
	paths := []string{}
	for _, name := range []string{"trip", "family"} {
		folder := filepath.Join(dir, name)
		if err := os.Mkdir(folder, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(folder, "photo.jpg")
		if err := os.WriteFile(path, []byte{}, 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	folders, err := (&ConfigManager{}).PreviewAutoAlbums(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 2 || folders[0] != filepath.Dir(paths[0]) || folders[1] != filepath.Dir(paths[1]) {
		t.Fatalf("unexpected preview: %v", folders)
	}
}

func TestCountUploadCandidatesEmptyAndMissing(t *testing.T) {
	manager := &ConfigManager{}
	dir := t.TempDir()
	count, err := manager.CountUploadCandidates([]string{dir})
	if err != nil || count != 0 {
		t.Fatalf("empty directory: count=%d error=%v", count, err)
	}
	if _, err := manager.CountUploadCandidates([]string{filepath.Join(dir, "missing")}); err == nil {
		t.Fatal("missing source must surface an error")
	}
}
