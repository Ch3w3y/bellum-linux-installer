package packages

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
)

func TestEnsureProtonKeepsVerifiedTreePristine(t *testing.T) {
	tmpDir := t.TempDir()
	protonDir := filepath.Join(tmpDir, "proton-test")
	if err := os.MkdirAll(protonDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"proton":                  "script",
		"user_settings.sample.py": "user_settings = {}\n",
		// Written by older installers into the shared tree.
		"user_settings.py": "user_settings = {\"PROTON_NVIDIA_LIBS\": \"1\"}\n",
	} {
		if err := os.WriteFile(filepath.Join(protonDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeProtonStamp(protonDir); err != nil {
		t.Fatal(err)
	}
	logger, _ := core.NewLogger("")
	// A verified tree is reused (no download) and the legacy settings go.
	if err := EnsureProton(tmpDir, "proton-test", logger); err != nil {
		t.Fatalf("EnsureProton failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(protonDir, "user_settings.py")); !os.IsNotExist(err) {
		t.Fatalf("legacy user_settings.py kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(protonDir, "user_settings.sample.py")); err != nil {
		t.Fatalf("sample settings must stay untouched: %v", err)
	}
	if err := verifyProtonStamp(protonDir); err != nil {
		t.Fatalf("tree not restamped: %v", err)
	}
}

func TestEnsureProtonReplacesUnverifiedTree(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	old, oldBase := downloadClient, config.DefaultVersions.ProtonBaseURL
	downloadClient, config.DefaultVersions.ProtonBaseURL = srv.Client(), srv.URL
	defer func() { downloadClient, config.DefaultVersions.ProtonBaseURL = old, oldBase }()

	tmpDir := t.TempDir()
	protonDir := filepath.Join(tmpDir, "proton-test")
	if err := os.MkdirAll(protonDir, 0755); err != nil {
		t.Fatal(err)
	}
	logger, _ := core.NewLogger("")
	err := EnsureProton(tmpDir, "proton-test", logger)
	if err == nil || !strings.Contains(err.Error(), "download") {
		t.Fatalf("expected a download error for an unverified tree, got %v", err)
	}
	if _, err := os.Stat(protonDir); !os.IsNotExist(err) {
		t.Fatalf("unverified tree kept: %v", err)
	}
}

func TestProtonStampDetectsTamperingAndPartialTrees(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "proton"), []byte("good"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyProtonStamp(root); err == nil {
		t.Fatal("unstamped partial tree trusted")
	}
	if err := writeProtonStamp(root); err != nil {
		t.Fatal(err)
	}
	if err := verifyProtonStamp(root); err != nil {
		t.Fatalf("fresh stamp rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "proton"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyProtonStamp(root); err == nil {
		t.Fatal("tampered tree trusted")
	}
}

func TestProtonStampIncludesVersionAndArchivePin(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "proton"), []byte("tree"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeProtonStamp(root); err != nil {
		t.Fatal(err)
	}
	oldVer, oldPin := config.DefaultVersions.ProtonVer, config.DefaultVersions.ProtonSHA256
	defer func() { config.DefaultVersions.ProtonVer, config.DefaultVersions.ProtonSHA256 = oldVer, oldPin }()
	config.DefaultVersions.ProtonSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := verifyProtonStamp(root); err == nil {
		t.Fatal("cache accepted after Proton pin change")
	}
	config.DefaultVersions.ProtonSHA256 = oldPin
	config.DefaultVersions.ProtonVer = oldVer + "-changed"
	if err := verifyProtonStamp(root); err == nil {
		t.Fatal("cache accepted after Proton version change")
	}
}
