package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
)

// playbellum.com/download redirects to this official release endpoint.
const launcherInstallerURL = "https://releases.astarte.industries/astartelauncher/windows-amd64/AstarteLauncher-amd64-installer.exe"

// LauncherInstallerState tracks the state of the launcher installer download
type LauncherInstallerState struct {
	InstallerPath string
	Downloaded    bool
	DownloadDir   string
}

// GetProtonURL returns the download URL for AMD/CachyOS Proton
func GetProtonURL(protonVer, protonBaseURL string) string {
	// Extract the directory prefix by stripping "proton-" prefix and "-x86_64" suffix
	prefix := protonVer
	if len(prefix) > 7 && prefix[:7] == "proton-" {
		prefix = prefix[7:]
	}
	if len(prefix) > 7 && prefix[len(prefix)-7:] == "-x86_64" {
		prefix = prefix[:len(prefix)-7]
	}
	// Use the full protonVer for the filename, prefix for the directory
	return fmt.Sprintf("%s/%s/%s.tar.xz", protonBaseURL, prefix, protonVer)
}

// DownloadLauncherInstaller downloads the launcher installer to a cache directory
func DownloadLauncherInstaller(workdir string, logger *core.Logger) (*LauncherInstallerState, error) {
	if config.DefaultVersions.LauncherSigner == "" {
		return nil, fmt.Errorf("launcher Authenticode signer pin is required before download")
	}
	filename := "AstarteLauncher-amd64-installer.exe"
	downloadDir, err := os.MkdirTemp("", "bellum-launcher-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create private download directory: %w", err)
	}
	if err := os.Chmod(downloadDir, 0700); err != nil {
		os.RemoveAll(downloadDir)
		return nil, err
	}
	dest := filepath.Join(downloadDir, filename)
	if err := os.MkdirAll(filepath.Join(workdir, "logs"), 0700); err != nil {
		os.RemoveAll(downloadDir)
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logger.Info(fmt.Sprintf("Downloading AstarteLauncher installer: %s", filename))

	logFile := filepath.Join(workdir, "logs", "installer.log")
	// Create the logs directory
	if err := downloadFile(dest, launcherInstallerURL, logFile, logger); err != nil {
		os.RemoveAll(downloadDir)
		logger.Error("Failed to download launcher installer")
		return nil, fmt.Errorf("failed to download launcher installer: %w", err)
	}

	if _, err := os.Stat(dest); os.IsNotExist(err) {
		os.RemoveAll(downloadDir)
		logger.Error("Download verification failed: launcher installer not found")
		return nil, fmt.Errorf("download verification failed: launcher installer not found")
	}
	check, err := VerifyLauncherInstaller(dest)
	if err != nil {
		os.RemoveAll(downloadDir)
		return nil, err
	}
	if warning := check.Warning(); warning != "" {
		logger.Warn(warning)
	}

	return &LauncherInstallerState{
		InstallerPath: dest,
		Downloaded:    true,
		DownloadDir:   downloadDir,
	}, nil
}

// CleanupLauncherInstaller removes the downloaded launcher installer
func CleanupLauncherInstaller(state *LauncherInstallerState, logger *core.Logger) error {
	if !state.Downloaded || state.InstallerPath == "" {
		return nil
	}

	logger.Info("Cleaning up downloaded launcher installer...")

	if err := os.Remove(state.InstallerPath); err != nil && !os.IsNotExist(err) {
		logger.Warn(fmt.Sprintf("Failed to remove launcher installer: %v", err))
	}

	if state.DownloadDir != "" {
		if err := os.Remove(state.DownloadDir); err != nil && !os.IsNotExist(err) {
			logger.Warn(fmt.Sprintf("Failed to remove launcher installer directory: %v (directory not empty or does not exist)", err))
		}
	}

	return nil
}

// // GetProtonURL returns the download URL for Proton (same for AMD and NVIDIA)
// func GetProtonURL(protonVer, protonBaseURL string) string {
// 	return Get(protonVer, protonBaseURL)
// }

// GetLocalProtonPath returns the path to a local Proton package if it exists
func GetLocalProtonPath(workdir, protonVer string) string {
	localProtonPath := filepath.Join(workdir, "packages", protonVer+".tar.gz")
	if _, err := os.Stat(localProtonPath); err == nil {
		return localProtonPath
	}
	return ""
}

// GetProtonInstallPath returns the path to the proton install directory
func GetProtonInstallPath(protonVer string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	return filepath.Join(homeDir, ".local", "share", "bellum", "proton", fmt.Sprintf("bellum-%s", protonVer))
}

// EnsureProton downloads and sets up the Proton directory
func EnsureProton(protonDir, protonVer string, isAMD bool, isFSR41 bool, logger *core.Logger) error {
	return EnsureProtonWithLog(filepath.Join(protonDir, protonVer), protonVer, isAMD, isFSR41, "", logger)
}

func EnsureProtonWithLog(protonDir, protonVer string, isAMD bool, isFSR41 bool, logPath string, logger *core.Logger) error {
	if len(config.DefaultVersions.ProtonSHA256) != 64 {
		return fmt.Errorf("approved Proton SHA-256 pin is required")
	}
	actualProtonDir := protonDir
	settingsFile := GetProtonUserSettingsPath(actualProtonDir)
	if settingsFile != "" && verifyProtonStamp(actualProtonDir) == nil {
		if err := PatchProtonSettings(settingsFile, isAMD, isFSR41); err != nil {
			return err
		}
		return writeProtonStamp(actualProtonDir)
	}
	parent := filepath.Dir(actualProtonDir)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	if err := os.RemoveAll(actualProtonDir); err != nil {
		return fmt.Errorf("remove unverified Proton tree: %w", err)
	}
	private, err := os.MkdirTemp("", "bellum-proton-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(private)
	if err := os.Chmod(private, 0700); err != nil {
		return err
	}
	archivePath := filepath.Join(private, protonVer+".tar.xz")
	if err := downloadFile(archivePath, GetProtonURL(protonVer, config.DefaultVersions.ProtonBaseURL), logPath, logger); err != nil {
		return err
	}
	if err := VerifySHA256(archivePath, config.DefaultVersions.ProtonSHA256); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".proton-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := ExtractPackageTo(archivePath, stage, 1); err != nil {
		return fmt.Errorf("failed to extract Proton: %w", err)
	}
	settingsFile = GetProtonUserSettingsPath(stage)
	if settingsFile == "" {
		return fmt.Errorf("Proton user settings file missing after extraction")
	}
	if err := PatchProtonSettings(settingsFile, isAMD, isFSR41); err != nil {
		return fmt.Errorf("failed to patch Proton user settings: %w", err)
	}
	if err := writeProtonStamp(stage); err != nil {
		return err
	}
	if err := os.Rename(stage, actualProtonDir); err != nil {
		return fmt.Errorf("atomically install Proton: %w", err)
	}
	return nil
}

// copyDirectory copies a directory recursively
func copyDirectory(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstDir, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

const protonStampName = ".bellum-verified-sha256"

func protonTreeDigest(root string) (string, error) {
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == protonStampName {
			return nil
		}
		if rel != "." {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	// Walk is lexical, and sorting makes the serialized digest explicit.
	sort.Strings(paths)
	h := sha256.New()
	for _, rel := range paths {
		path := filepath.Join(root, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, rel+"\x00"+info.Mode().String()+"\x00")
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			_, _ = io.WriteString(h, link)
		} else if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return "", err
			}
			_, copyErr := io.Copy(h, f)
			closeErr := f.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
		} else if !info.IsDir() {
			return "", fmt.Errorf("unsupported file in Proton tree: %s", rel)
		}
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeProtonStamp(root string) error {
	digest, err := protonTreeDigest(root)
	if err != nil {
		return err
	}
	tmp := filepath.Join(root, protonStampName+".tmp")
	stamp := protonStamp(digest)
	if err := os.WriteFile(tmp, []byte(stamp), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(root, protonStampName))
}

func verifyProtonStamp(root string) error {
	stamp, err := os.ReadFile(filepath.Join(root, protonStampName))
	if err != nil {
		return err
	}
	got, err := protonTreeDigest(root)
	if err != nil {
		return err
	}
	if string(stamp) != protonStamp(got) {
		return fmt.Errorf("Proton cache integrity stamp mismatch")
	}
	return nil
}

func protonStamp(treeDigest string) string {
	return fmt.Sprintf("version=%s\nsha256=%s\ntree=%s\n", config.DefaultVersions.ProtonVer, config.DefaultVersions.ProtonSHA256, treeDigest)
}

// downloadFile fetches url into dest (which must not exist) over HTTPS with
// Go's HTTP client, so no wget or curl is needed. It honours the usual proxy
// environment variables and prints coarse progress for large files.
func downloadFile(dest, url, logFile string, logger *core.Logger) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("refusing to download over existing path")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := fetchURL(dest, url, logger); err != nil {
		logger.Error(fmt.Sprintf("Failed to download %s", url))
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	return nil
}

// downloadClient allows slow connections but not a stalled one forever.
var downloadClient = &http.Client{Timeout: 2 * time.Hour}

func fetchURL(dest, url string, logger *core.Logger) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("refusing non-HTTPS download")
	}
	resp, err := downloadClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server answered %s", resp.Status)
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	progress := &downloadProgress{total: resp.ContentLength, logger: logger, name: filepath.Base(dest)}
	_, copyErr := io.Copy(f, io.TeeReader(resp.Body, progress))
	closeErr := f.Close()
	if err := firstErr(copyErr, closeErr); err != nil {
		os.Remove(dest)
		return err
	}
	if resp.ContentLength > 0 && progress.done != resp.ContentLength {
		os.Remove(dest)
		return fmt.Errorf("download truncated: got %d of %d bytes", progress.done, resp.ContentLength)
	}
	return nil
}

type downloadProgress struct {
	total, done int64
	lastPercent int
	name        string
	logger      *core.Logger
}

func (p *downloadProgress) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.total > 50<<20 { // only report for large downloads
		if pct := int(p.done * 100 / p.total); pct >= p.lastPercent+10 {
			p.lastPercent = pct - pct%10
			p.logger.Info(fmt.Sprintf("Downloading %s: %d%% of %d MB", p.name, p.lastPercent, p.total>>20))
		}
	}
	return len(b), nil
}
