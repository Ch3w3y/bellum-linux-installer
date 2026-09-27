package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The six files in Valve's public EAC runtime depot 1826331, manifest
// 3310269496439035229. Hash names and bytes to make verification independent
// of directory metadata and the user's Steam library location.
var eacRuntimeFiles = []string{
	"v2/lib32/easyanticheat_x86.dll",
	"v2/lib32/easyanticheat_x86.so",
	"v2/lib64/easyanticheat.dll",
	"v2/lib64/easyanticheat.so",
	"v2/lib64/easyanticheat_x64.dll",
	"v2/lib64/easyanticheat_x64.so",
}

func EACRuntimeDigest(root string) (string, error) {
	h := sha256.New()
	for _, name := range eacRuntimeFiles {
		path := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("EAC runtime file missing or unsafe: %s", name)
		}
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, _ = h.Write([]byte(name + "\x00"))
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func VerifyEACRuntime(root string, approved []string) error {
	if len(approved) == 0 {
		return fmt.Errorf("approved EAC runtime SHA-256 pin is required")
	}
	got, err := EACRuntimeDigest(root)
	if err != nil {
		return err
	}
	for _, expected := range approved {
		if len(expected) == 64 && strings.EqualFold(got, expected) {
			return nil
		}
	}
	return fmt.Errorf("EAC runtime digest mismatch")
}
