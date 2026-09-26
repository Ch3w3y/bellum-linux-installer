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
}
