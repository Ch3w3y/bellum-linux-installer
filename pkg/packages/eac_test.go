package packages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEACRuntimeVerification(t *testing.T) {
	root := t.TempDir()
	if _, _, err := VerifyEACRuntime(root, nil); err == nil {
		t.Fatal("runtime with missing files was accepted")
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
	if got, known, err := VerifyEACRuntime(root, []string{pin, strings.Repeat("0", 64)}); err != nil || !known || got != pin {
		t.Fatalf("approved runtime: digest=%s known=%t err=%v", got, known, err)
	}
	// A Steam update changes the digest: still accepted, but reported unknown.
	path := filepath.Join(root, filepath.FromSlash(eacRuntimeFiles[0]))
	if err := os.WriteFile(path, []byte("updated by Steam"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, known, err := VerifyEACRuntime(root, []string{pin}); err != nil || known {
		t.Fatalf("updated runtime: known=%t err=%v", known, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyEACRuntime(root, []string{pin}); err == nil {
		t.Fatal("runtime missing a file was accepted")
	}
}
