package packages

import (
	"fmt"
	"os"
	"path/filepath"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
)

const umuReleaseBase = "https://github.com/Open-Wine-Components/umu-launcher/releases/download"

// UMUInstallDir is where the pinned umu-launcher zipapp is kept.
func UMUInstallDir(version string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".local", "share", "bellum", "umu", version)
}

// EnsureUMU installs the pinned umu-launcher zipapp into dir (if it isn't
// there already with the right contents) and returns the path to umu-run.
// The zipapp bundles its Python dependencies and needs only python3.
func EnsureUMU(dir string, logger *core.Logger) (string, error) {
	return ensureUMUFrom(dir, fmt.Sprintf("%s/%s/umu-launcher-%s-zipapp.tar", umuReleaseBase, config.DefaultVersions.UMUVersion, config.DefaultVersions.UMUVersion), config.DefaultVersions.UMUZipappSHA256, logger)
}

func ensureUMUFrom(dir, url, pin string, logger *core.Logger) (string, error) {
	if len(pin) != 64 {
		return "", fmt.Errorf("approved umu-launcher SHA-256 pin is required")
	}
	umuRun := filepath.Join(dir, "umu-run")
	stamp := filepath.Join(dir, ".bellum-verified-sha256")
	if got, err := os.ReadFile(stamp); err == nil && string(got) == pin {
		if info, err := os.Stat(umuRun); err == nil && info.Mode().IsRegular() && info.Mode()&0100 != 0 {
			return umuRun, nil
		}
	}

	logger.Info("Downloading umu-launcher " + config.DefaultVersions.UMUVersion + "...")
	private, err := os.MkdirTemp("", "bellum-umu-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(private)
	archive := filepath.Join(private, "umu.tar")
	if err := downloadFile(archive, url, "", logger); err != nil {
		return "", err
	}
	if err := VerifySHA256(archive, pin); err != nil {
		return "", err
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(parent, ".umu-stage-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if err := ExtractPackageTo(archive, stage, 1); err != nil {
		return "", fmt.Errorf("unpack umu-launcher: %w", err)
	}
	staged := filepath.Join(stage, "umu-run")
	info, err := os.Lstat(staged)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("umu-launcher archive has no umu-run")
	}
	if err := os.Chmod(staged, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(stage, ".bellum-verified-sha256"), []byte(pin), 0600); err != nil {
		return "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.Rename(stage, dir); err != nil {
		return "", fmt.Errorf("install umu-launcher: %w", err)
	}
	return umuRun, nil
}
