package packages

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"bellum-installer/pkg/core"
)

// The release list as served on 2026-09-27 (trimmed).
const sampleReleases = `v1.4.2 0 https://releases.astarte.industries/astartelauncher/windows-amd64/1.4.2/AstarteLauncher.exe 75966072
v1.4.1 0 https://releases.astarte.industries/astartelauncher/windows-amd64/1.4.1/AstarteLauncher.exe 75966072
v1.3.3 0 https://releases.astarte.industries/astartelauncher/windows-amd64/1.3.3/AstarteLauncher.exe 75966072
v1.1.2-alpha0 0 https://releases.astarte.industries/astartelauncher/windows-amd64/1.1.2-alpha0/AstarteLauncher.exe 75966072
`

func TestParseLauncherReleasesPicksNewestStable(t *testing.T) {
	rel, err := parseLauncherReleases([]byte(sampleReleases))
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "v1.4.2" || rel.URL != launcherReleaseBase+"1.4.2/AstarteLauncher.exe" {
		t.Fatalf("got %+v", rel)
	}
	// Order doesn't matter, and 1.10.0 is newer than 1.9.9.
	rel, err = parseLauncherReleases([]byte("v1.9.9 0 " + launcherReleaseBase + "1.9.9/AstarteLauncher.exe 1\nv1.10.0 0 " + launcherReleaseBase + "1.10.0/AstarteLauncher.exe 1\n"))
	if err != nil || rel.Version != "v1.10.0" {
		t.Fatalf("got %+v, %v", rel, err)
	}
}

func TestParseLauncherReleasesRejectsForeignOrMismatchedURLs(t *testing.T) {
	for _, body := range []string{
		"v9.9.9 0 https://evil.example/astartelauncher/windows-amd64/9.9.9/AstarteLauncher.exe 1\n",
		"v9.9.9 0 http://releases.astarte.industries/astartelauncher/windows-amd64/9.9.9/AstarteLauncher.exe 1\n",
		"v9.9.9 0 " + launcherReleaseBase + "1.4.2/AstarteLauncher.exe 1\n",
		"v9.9.9 0 " + launcherReleaseBase + "9.9.9/../../other.exe 1\n",
		"v1.2.3-beta 0 " + launcherReleaseBase + "1.2.3-beta/AstarteLauncher.exe 1\n",
		"v01.2.3 0 " + launcherReleaseBase + "01.2.3/AstarteLauncher.exe 1\n",
		"",
	} {
		if rel, err := parseLauncherReleases([]byte(body)); err == nil {
			t.Errorf("accepted %q as %+v", body, rel)
		}
	}
}

func TestUpdateLauncherReplacesVerifiesAndSkipsWhenCurrent(t *testing.T) {
	newExe := []byte("MZ new launcher build")
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(newExe)
	}))
	defer srv.Close()
	old := downloadClient
	downloadClient = srv.Client()
	defer func() { downloadClient = old }()

	logger, _ := core.NewLogger("")
	dir := t.TempDir()
	exe := filepath.Join(dir, "AstarteLauncher.exe")
	stamp := filepath.Join(dir, "stamp")
	if err := os.WriteFile(exe, []byte("MZ old build"), 0755); err != nil {
		t.Fatal(err)
	}
	rel := LauncherRelease{Version: "v1.4.2", URL: srv.URL + "/1.4.2/AstarteLauncher.exe"}
	ok := func(string) error { return nil }

	// A bad signature keeps the installed launcher.
	if _, _, err := updateLauncherTo(rel, exe, stamp, logger, func(string) error { return errors.New("bad signer") }); err == nil {
		t.Fatal("unsigned launcher accepted")
	}
	if b, _ := os.ReadFile(exe); string(b) != "MZ old build" {
		t.Fatal("launcher replaced despite failed signature check")
	}

	version, replaced, err := updateLauncherTo(rel, exe, stamp, logger, ok)
	if err != nil || !replaced || version != "v1.4.2" {
		t.Fatalf("update: %v %v %v", version, replaced, err)
	}
	if b, _ := os.ReadFile(exe); string(b) != string(newExe) {
		t.Fatal("launcher not replaced")
	}
	if info, _ := os.Stat(exe); info.Mode().Perm()&0100 == 0 {
		t.Fatalf("launcher not executable: %v", info.Mode())
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".AstarteLauncher-") {
			t.Fatalf("temporary download left behind: %s", e.Name())
		}
	}

	before := hits.Load()
	if _, replaced, err := updateLauncherTo(rel, exe, stamp, logger, ok); err != nil || replaced {
		t.Fatalf("second run: %v %v", replaced, err)
	}
	if hits.Load() != before {
		t.Fatal("up-to-date launcher downloaded again")
	}

	// The launcher changed on disk (e.g. replaced by hand): check again.
	if err := os.WriteFile(exe, []byte("MZ something else"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, replaced, err := updateLauncherTo(rel, exe, stamp, logger, ok); err != nil || !replaced {
		t.Fatalf("changed launcher not refreshed: %v %v", replaced, err)
	}
}
