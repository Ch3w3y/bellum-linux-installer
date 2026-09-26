package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBellumPrefixRequiresMarkersAndRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	for _, path := range []string{"/", root, filepath.Join(root, "Other")} {
		if err := validateBellumPrefix(path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	prefix := filepath.Join(root, "Bellum")
	if err := os.MkdirAll(filepath.Join(prefix, "drive_c"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := validateBellumPrefix(prefix); err == nil {
		t.Fatal("accepted prefix without registry marker")
	}
	if err := os.WriteFile(filepath.Join(prefix, "system.reg"), []byte("WINE REGISTRY Version 2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(prefix, DefaultBoundaries.Files); err != nil {
		t.Fatal(err)
	}
	if err := validateBellumPrefix(prefix); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(prefix, link); err != nil {
		t.Fatal(err)
	}
	if err := validateBellumPrefix(link); err == nil {
		t.Fatal("accepted symlink")
	}
}
