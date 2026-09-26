package packages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEACRuntimeVerificationFailsClosed(t *testing.T) {
	root := t.TempDir()
	if err := VerifyEACRuntime(root, ""); err == nil {
		t.Fatal("missing pin accepted")
	}
	for _, name := range eacRuntimeFiles {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	pin, err := EACRuntimeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyEACRuntime(root, pin); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(eacRuntimeFiles[0]))
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyEACRuntime(root, pin); err == nil {
		t.Fatal("tampered runtime accepted")
	}
}
