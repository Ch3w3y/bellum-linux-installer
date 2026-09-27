package packages

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bellum-installer/pkg/core"
)

// The Astarte Launcher updates itself by downloading the new AstarteLauncher.exe
// and swapping it for the running one. Under Wine that swap fails while the
// launcher still logs success and restarts, so it finds the same update again
// and loops. Bellum therefore installs launcher updates from Linux, verified
// with the same Authenticode signer check as the launcher installer, before
// each launch; the launcher then starts up to date and never tries its own.

// LauncherReleasesURL lists launcher releases, newest first, one per line:
// "v1.4.2 0 https://…/1.4.2/AstarteLauncher.exe 75966072". The last field is
// not the file size (every line carries the same value), so it is ignored.
const LauncherReleasesURL = "https://releases.astarte.industries/astartelauncher/windows-amd64/RELEASES"

const launcherReleaseBase = "https://releases.astarte.industries/astartelauncher/windows-amd64/"

// launcherStampName records, in the prefix, which release Bellum installed
// and the SHA-256 of the exe it wrote.
const launcherStampName = ".bellum-launcher-release"

// LauncherRelease is one entry of the release list.
type LauncherRelease struct {
	Version string // "v1.4.2"
	URL     string
	parts   [3]int
}

// parseLauncherReleases returns the newest stable release in the list.
// Pre-releases (a "-" suffix) and entries whose URL isn't the official
// per-version AstarteLauncher.exe are skipped.
func parseLauncherReleases(body []byte) (LauncherRelease, error) {
	var best LauncherRelease
	found := false
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		rel, ok := parseLauncherRelease(fields[0], fields[2])
		if !ok {
			continue
		}
		if !found || newerRelease(rel.parts, best.parts) {
			best, found = rel, true
		}
	}
	if err := sc.Err(); err != nil {
		return LauncherRelease{}, err
	}
	if !found {
		return LauncherRelease{}, fmt.Errorf("the launcher release list has no stable release")
	}
	return best, nil
}

func parseLauncherRelease(version, url string) (LauncherRelease, bool) {
	num, ok := strings.CutPrefix(version, "v")
	if !ok || strings.Contains(num, "-") {
		return LauncherRelease{}, false
	}
	segs := strings.Split(num, ".")
	if len(segs) != 3 {
		return LauncherRelease{}, false
	}
	var parts [3]int
	for i, s := range segs {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 || strconv.Itoa(n) != s {
			return LauncherRelease{}, false
		}
		parts[i] = n
	}
	if url != launcherReleaseBase+num+"/AstarteLauncher.exe" {
		return LauncherRelease{}, false
	}
	return LauncherRelease{Version: version, URL: url, parts: parts}, true
}

func newerRelease(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

var releaseListClient = &http.Client{Timeout: 30 * time.Second}

// LatestLauncherRelease fetches the release list and returns its newest
// stable release.
func LatestLauncherRelease() (LauncherRelease, error) {
	resp, err := releaseListClient.Get(LauncherReleasesURL)
	if err != nil {
		return LauncherRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return LauncherRelease{}, fmt.Errorf("launcher release list: server answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return LauncherRelease{}, err
	}
	return parseLauncherReleases(body)
}

// UpdateLauncher makes the AstarteLauncher.exe at exePath the latest stable
// release, verified by its Authenticode signature. stampPath records what
// was installed so later calls skip the download. It reports the version now
// installed and whether the exe was replaced.
func UpdateLauncher(exePath, stampPath string, logger *core.Logger) (string, bool, error) {
	rel, err := LatestLauncherRelease()
	if err != nil {
		return "", false, fmt.Errorf("check for launcher updates: %w", err)
	}
	return updateLauncherTo(rel, exePath, stampPath, logger, VerifyLauncherAuthenticode)
}

func updateLauncherTo(rel LauncherRelease, exePath, stampPath string, logger *core.Logger, verify func(string) error) (string, bool, error) {
	info, err := os.Lstat(exePath)
	if err != nil {
		return "", false, fmt.Errorf("launcher not installed: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("launcher is not a regular file: %s", exePath)
	}
	current, err := fileSHA256(exePath)
	if err != nil {
		return "", false, err
	}
	if stamp, err := os.ReadFile(stampPath); err == nil && string(stamp) == launcherStamp(rel.Version, current) {
		return rel.Version, false, nil
	}

	dir := filepath.Dir(exePath)
	tmp := filepath.Join(dir, fmt.Sprintf(".AstarteLauncher-%s-%d.exe", rel.Version, os.Getpid()))
	_ = os.Remove(tmp)
	if err := fetchURL(tmp, rel.URL, logger); err != nil {
		return "", false, fmt.Errorf("download launcher %s: %w", rel.Version, err)
	}
	defer os.Remove(tmp)
	if err := verify(tmp); err != nil {
		return "", false, fmt.Errorf("launcher %s failed its signature check, keeping the installed one: %w", rel.Version, err)
	}
	digest, err := fileSHA256(tmp)
	if err != nil {
		return "", false, err
	}
	replaced := digest != current
	if replaced {
		if err := os.Chmod(tmp, info.Mode().Perm()|0o100); err != nil {
			return "", false, err
		}
		if err := os.Rename(tmp, exePath); err != nil {
			return "", false, fmt.Errorf("install launcher %s: %w", rel.Version, err)
		}
	}
	if err := os.WriteFile(stampPath, []byte(launcherStamp(rel.Version, digest)), 0600); err != nil {
		return "", false, fmt.Errorf("record launcher version: %w", err)
	}
	return rel.Version, replaced, nil
}

func launcherStamp(version, digest string) string {
	return fmt.Sprintf("version=%s\nsha256=%s\n", version, digest)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// LauncherStampPath is where the installed launcher release is recorded.
func LauncherStampPath(prefix string) string {
	return filepath.Join(prefix, launcherStampName)
}

// ToolPath is the stable copy of the installer that the Bellum wrapper runs
// before each launch to install launcher updates.
func ToolPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".local", "share", "bellum", "bin", "bellum-installer")
}

// InstallTool copies the running installer binary to ToolPath.
func InstallTool() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dest := ToolPath()
	if resolved, err := filepath.EvalSymlinks(self); err == nil && resolved == dest {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	src, err := os.Open(self)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".bellum-installer-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0700); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}
