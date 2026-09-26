package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// VerifySHA256 refuses missing or mismatched pins before an artifact is used.
func VerifySHA256(path, expected string) error {
	if len(expected) != 64 {
		return fmt.Errorf("missing SHA-256 pin for %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("SHA-256 mismatch for %s", path)
	}
	return nil
}

const (
	WinetricksSHA256 = "ca1d0a5f018412c6d92ed6c615aba27416c8ec00417831bf8c0a85d8b03a14a2"
	IconSHA256       = "39646334a10452a17537b86b72c480d5a3cdd1a5af5585d8bc86122e411d00df"
)
