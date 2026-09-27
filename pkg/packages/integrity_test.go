package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySHA256FailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("approved"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, ""); err == nil {
		t.Fatal("missing pin accepted")
	}
	if err := VerifySHA256(path, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("wrong pin accepted")
	}
	hash := sha256.Sum256([]byte("approved"))
	if err := VerifySHA256(path, hex.EncodeToString(hash[:])); err != nil {
		t.Fatal(err)
	}
	// A file substituted after the digest is approved must be rejected at use.
	if err := os.WriteFile(path, []byte("substituted"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, hex.EncodeToString(hash[:])); err == nil {
		t.Fatal("substituted artifact accepted")
	}
}

func TestCopyVerifiedSHA256UsesPrivateDestinationBytes(t *testing.T) {
	dir := t.TempDir()
	source, staged := filepath.Join(dir, "source"), filepath.Join(dir, "staged")
	if err := os.WriteFile(source, []byte("approved"), 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("approved"))
	if err := CopyVerifiedSHA256(source, staged, hex.EncodeToString(hash[:])); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "approved" {
		t.Fatalf("staged copy = %q", got)
	}
}
