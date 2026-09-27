package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagedLauncherIsUnaffectedBySourceReplacement(t *testing.T) {
	source := filepath.Join(t.TempDir(), "launcher.exe")
	if err := os.WriteFile(source, []byte("approved bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("approved bytes"))
	staged, dir, check, err := stageLauncherInstaller(source, []string{hex.EncodeToString(digest[:])}, func(path string) error {
		if path == source {
			t.Fatal("verification ran on the source path")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if !check.KnownDigest || check.Warning() != "" {
		t.Fatalf("allowlisted digest reported unknown: %+v", check)
	}
	if err := os.WriteFile(source, []byte("attacker bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "approved bytes" {
		t.Fatalf("staged bytes changed after source replacement: %q", got)
	}
}

func TestUnknownLauncherDigestWithValidSignerIsAcceptedWithWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launcher.exe")
	if err := os.WriteFile(path, []byte("next Astarte release"), 0600); err != nil {
		t.Fatal(err)
	}
	check, err := verifyLauncherInstaller(path, []string{strings.Repeat("0", 64)}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("signed build with unknown digest rejected: %v", err)
	}
	if check.KnownDigest || !strings.Contains(check.Warning(), check.Digest) {
		t.Fatalf("unknown digest not reported: %+v %q", check, check.Warning())
	}
	if _, err := verifyLauncherInstaller(path, nil, func(string) error { return errors.New("bad signature") }); err == nil {
		t.Fatal("an invalid signature must always fail")
	}
}
