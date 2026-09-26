package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestAuthorizesOnlyMatchingInstanceAndRollbackIsScoped(t *testing.T) {
	root := t.TempDir()
	prefix := filepath.Join(root, "Bellum")
	if err := os.MkdirAll(filepath.Join(prefix, "drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "system.reg"), []byte("wine"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(prefix, DefaultBoundaries.Files); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(prefix, DefaultBoundaries.Files); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "Other")
	if err := os.Mkdir(other, 0700); err != nil {
		t.Fatal(err)
	}
	if err := rollbackNewPrefix(prefix, false, DefaultBoundaries.Files); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(prefix); err != nil {
		t.Fatalf("pre-existing prefix removed: %v", err)
	}
	if err := rollbackNewPrefix(prefix, true, DefaultBoundaries.Files); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(prefix); !os.IsNotExist(err) {
		t.Fatalf("new prefix remains: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("unrelated directory removed: %v", err)
	}
}

func TestReadManifestRejectsDifferentPrefix(t *testing.T) {
	root := t.TempDir()
	prefix := filepath.Join(root, "Bellum")
	if err := os.Mkdir(prefix, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(prefix, DefaultBoundaries.Files); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(prefix, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(`{"version":1,"prefix":"/","owned":["prefix"]}`)
	if err := os.WriteFile(filepath.Join(prefix, manifestName), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(prefix, DefaultBoundaries.Files); err == nil {
		t.Fatal("accepted manifest for another prefix")
	}
}
