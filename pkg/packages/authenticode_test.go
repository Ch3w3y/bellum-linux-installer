package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestApprovedSignerMustBeLeafExactCN(t *testing.T) {
	fixtures := []string{
		"Signer #0:\n Subject: /C=US/ST=Delaware/L=Newark/O=ASTARTE INDUSTRIES INC./CN=ASTARTE INDUSTRIES INC.\nSigner #1:\n Subject: /C=US/O=ASTARTE INDUSTRIES INC./CN=ASTARTE INDUSTRIES INC.\n", // osslsigncode 2.5
		"Signer #0:\n Subject: /C=US/ST=Delaware/L=Newark/O=ASTARTE INDUSTRIES INC./CN=ASTARTE INDUSTRIES INC.\nSigner #1:\n Subject: /C=US/O=ASTARTE INDUSTRIES INC./CN=ASTARTE INDUSTRIES INC.\n", // osslsigncode 2.9
		"Signer #0:\n Subject: C=US, ST=Delaware, L=Newark, O=ASTARTE INDUSTRIES INC., CN=ASTARTE INDUSTRIES INC.\nSigner #1:\n Subject: CN=ASTARTE INDUSTRIES INC.\n",                              // osslsigncode 2.10
	}
	for i, fixture := range fixtures {
		if !hasApprovedLeafSubject(fixture, "ASTARTE INDUSTRIES INC.") {
			t.Fatalf("osslsigncode output fixture %d rejected", i)
		}
	}
	valid := fixtures[2]
	if hasApprovedLeafSubject(valid, "ASTARTE") {
		t.Fatal("partial signer subject accepted")
	}
	if hasApprovedLeafSubject("Signer #0:\n Subject: CN=OTHER INC.\n", "ASTARTE INDUSTRIES INC.") {
		t.Fatal("wrong signer accepted")
	}
	for _, forged := range []string{
		"Signer #0:\n Subject: O=Evil\\, CN=ASTARTE INDUSTRIES INC.\n",
		"Signer #0:\n Subject: O=Evil/CN=ASTARTE INDUSTRIES INC.\n",
		"Signer #0:\n Subject: CN=ASTARTE INDUSTRIES INC., CN=ASTARTE INDUSTRIES INC.\n",
		"Signer #0:\n Subject: CN=astarte industries inc.\n",
	} {
		if hasApprovedLeafSubject(forged, "ASTARTE INDUSTRIES INC.") {
			t.Fatalf("forged or non-exact signer accepted: %s", forged)
		}
	}
}

func TestStagedLauncherIsUnaffectedBySourceReplacement(t *testing.T) {
	source := filepath.Join(t.TempDir(), "launcher.exe")
	if err := os.WriteFile(source, []byte("approved bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("approved bytes"))
	staged, dir, err := stageLauncherInstaller(source, []string{hex.EncodeToString(digest[:])}, func(path string) error {
		if path == source {
			t.Fatal("verification ran on the source path")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
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
