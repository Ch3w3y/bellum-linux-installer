package launchers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"bellum-installer/pkg/core"
)

// LauncherConfig holds configuration for launcher generation
type LauncherConfig struct {
	Wineprefix string
	Protonpath string
	GPUType    string
	IconPath   string
}

// GenerateLauncher generates the launcher wrapper scripts and desktop files.
func GenerateLauncher(config LauncherConfig) error {
	if err := generateLauncherWrapper(config); err != nil {
		return fmt.Errorf("failed to generate launcher wrapper: %w", err)
	}

	// Generate .desktop files and icon (common to both GPU types)
	if err := generateDesktopFiles(config); err != nil {
		return fmt.Errorf("failed to generate desktop files: %w", err)
	}

	return nil
}

// generateLauncherWrapper writes the shared umu launcher.
func generateLauncherWrapper(config LauncherConfig) error {
	script := generateWrapperContent(config)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return writeWrapper(filepath.Join(home, ".local", "bin", "Bellum"), script)
}

// generateWrapperContent builds the wrapper script content based on GPU type.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// All vendors use the same Proton/EAC launch path. The launcher log stays in
// the prefix, outside the game installation tree.
func generateWrapperContent(config LauncherConfig) string {
	launcherExe := filepath.Join(config.Wineprefix, "drive_c/users/steamuser/AppData/Local/Astarte Industries/Astarte Launcher/AstarteLauncher.exe")
	return fmt.Sprintf(`#!/bin/bash
set -eu
LAUNCH_VARS=%s
LAUNCHER_EXE=%s
[ -f "$LAUNCH_VARS" ] || { echo "Missing launch variables: $LAUNCH_VARS" >&2; exit 1; }
[ -f "$LAUNCHER_EXE" ] || { echo "Missing launcher: $LAUNCHER_EXE" >&2; exit 1; }
set -a
source "$LAUNCH_VARS"
set +a
[ -d "${PROTON_EAC_RUNTIME:-}" ] || { echo "Proton EasyAntiCheat Runtime is missing: ${PROTON_EAC_RUNTIME:-unset}" >&2; exit 1; }
[ -x "$PROTONPATH/proton" ] || { echo "Pinned Proton is missing: $PROTONPATH" >&2; exit 1; }
# The installer pins its own umu-launcher; fall back to one on PATH.
UMU_RUN="${BELLUM_UMU_RUN:-$(command -v umu-run || true)}"
[ -n "$UMU_RUN" ] && [ -x "$UMU_RUN" ] || { echo "umu-run is missing: ${UMU_RUN:-not found}. Re-run the Bellum installer." >&2; exit 1; }
command -v flock >/dev/null || { echo "flock is required" >&2; exit 1; }
umask 077
chmod 0700 "$WINEPREFIX"
chmod 0600 "$LAUNCH_VARS"
exec 9>"$WINEPREFIX/.bellum-launch.lock"
if ! flock -n 9; then
  echo "Bellum is already running in $WINEPREFIX" >&2
  exit 0
fi
touch "$WINEPREFIX/launcher.log"
chmod 0600 "$WINEPREFIX/launcher.log"
export GAMEID="${GAMEID:-nonsteam}"
export UMU_LOG=1
# Keep the container alive while the launcher and game share the Wine session.
export PROTON_VERB=waitforexitandrun
if [ "${BELLUM_MANGOHUD:-0}" = 1 ]; then export MANGOHUD=1; fi
if [ "${BELLUM_VKBASALT:-0}" = 1 ]; then export ENABLE_VKBASALT=1; fi
cmd=("$UMU_RUN" "$LAUNCHER_EXE" "$@")
if [ "${BELLUM_GAMEMODE:-0}" = 1 ]; then
  command -v gamemoderun >/dev/null || { echo "gamemoderun is required for BELLUM_GAMEMODE=1" >&2; exit 1; }
  cmd=(gamemoderun "${cmd[@]}")
fi
if [ "${BELLUM_GAMESCOPE:-0}" = 1 ]; then
  command -v gamescope >/dev/null || { echo "gamescope is required for BELLUM_GAMESCOPE=1" >&2; exit 1; }
  cmd=(gamescope -- "${cmd[@]}")
fi
exec "${cmd[@]}" >> "$WINEPREFIX/launcher.log" 2>&1
`, shellQuote(filepath.Join(config.Wineprefix, "launch_vars.env")), shellQuote(launcherExe))
}

// writeWrapper writes a wrapper script to the specified path.
func writeWrapper(path, content string) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	return nil
}

// generateDesktopFiles creates the .desktop files and icon for the launcher.
// Both GPU types use the same binary name "Bellum" and desktop file "Bellum.desktop".
// This mirrors the bash behavior in lib/launcher.sh:
//   - Desktop files go to ~/.local/share/applications/
//   - Desktop files are also copied to ~/Desktop/
//   - Icon goes to ~/.local/share/icons/hicolor/256x256/apps/bellum.png
//   - gio set metadata::trusted is applied if available
//   - update-desktop-database is run if available
func generateDesktopFiles(config LauncherConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	appsDir := filepath.Join(homeDir, ".local", "share", "applications")
	appsIconDir := filepath.Join(homeDir, ".local", "share", "icons", "hicolor", "256x256", "apps")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return fmt.Errorf("create applications directory: %w", err)
	}

	// The desktop entry does not depend on the GPU: every vendor, including
	// an unrecognised one, launches through the same umu wrapper.
	entryComment := "Launch Bellum via Proton and umu"
	entryName := "Bellum"
	entryExec := filepath.Join(homeDir, ".local", "bin", "Bellum")

	// Install icon
	installedIcon := ""
	if config.IconPath != "" {
		if err := os.MkdirAll(appsIconDir, 0755); err != nil {
			return fmt.Errorf("failed to create icon directory: %w", err)
		}
		iconDest := filepath.Join(appsIconDir, "bellum.png")
		if err := copyFile(config.IconPath, iconDest); err != nil {
			return fmt.Errorf("failed to copy icon: %w", err)
		}
		installedIcon = filepath.Join(appsIconDir, "bellum.png")
	} else {
		// Fallback: no icon
		installedIcon = ""
	}

	// Generate .desktop file
	desktopFile := filepath.Join(appsDir, fmt.Sprintf("%s.desktop", entryName))
	desktopContent := fmt.Sprintf(`[Desktop Entry]
Name=%s
Comment=%s
Exec=%s
Icon=%s
Type=Application
Categories=Game;
Terminal=false
Path=%s
`, entryName, entryComment, entryExec, installedIcon, config.Wineprefix)

	if err := os.WriteFile(desktopFile, []byte(desktopContent), 0644); err != nil {
		return fmt.Errorf("failed to write desktop file %s: %w", desktopFile, err)
	}

	// Copy to ~/Desktop/
	desktopDest := filepath.Join(homeDir, "Desktop", fmt.Sprintf("%s.desktop", entryName))
	if _, err := os.Stat(filepath.Join(homeDir, "Desktop")); err == nil {
		if err := copyFile(desktopFile, desktopDest); err != nil {
			// Non-fatal: desktop directory may not exist or be inaccessible
		}
	}

	// Set write perm
	if _, err := os.Stat(desktopDest); err == nil {
		if err := os.Chmod(desktopDest, 0644); err != nil {
			return fmt.Errorf("failed to set Bellum desktop file permissions: %w", err)
		}
	}

	// Mark as trusted via gio if available
	if runtime.GOOS == "linux" {
		_ = runGioSet(desktopFile, "metadata::trusted", "true")
		if _, err := os.Stat(desktopDest); err == nil {
			_ = runGioSet(desktopDest, "metadata::trusted", "true")
		}
	}

	// Update desktop database if available
	if runtime.GOOS == "linux" {
		_ = runUpdateDesktopDatabase(appsDir)
	}

	return nil
}

// runGioSet runs `gio set <file> <key> <value>` silently (non-fatal).
func runGioSet(file, key, value string) error {
	// Create a logger for this operation
	logger, err := core.NewLogger("")
	if err != nil {
		return err
	}
	defer logger.Close()
	return core.RunCommand(core.RunModeSilent, []string{"gio", "set", file, key, value}, logger, "")
}

// runUpdateDesktopDatabase runs `update-desktop-database <dir>` silently (non-fatal).
func runUpdateDesktopDatabase(dir string) error {
	// Create a logger for this operation
	logger, err := core.NewLogger("")
	if err != nil {
		return err
	}
	defer logger.Close()
	return core.RunCommand(core.RunModeSilent, []string{"update-desktop-database", dir}, logger, "")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destination.Close()

	_, err = destination.ReadFrom(source)
	return err
}

// CopyIcon copies the launcher icon to the user's local icon directory
// (matching bash behavior: ~/.local/share/icons/hicolor/256x256/apps/bellum.png)
func CopyIcon(iconPath string) error {
	if iconPath == "" {
		return fmt.Errorf("icon path is empty")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	iconDest := filepath.Join(homeDir, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png")
	if err := os.MkdirAll(filepath.Dir(iconDest), 0755); err != nil {
		return fmt.Errorf("failed to create icon directory: %w", err)
	}

	return copyFile(iconPath, iconDest)
}

// DetectLauncherBinary returns the launcher binary name for the given GPU type.
// Both AMD and NVIDIA use the same binary name "Bellum" — the difference is
// in the wrapper content (wine vs proton).
func DetectLauncherBinary(gpuType string) string {
	return "Bellum"
}

// IsLauncherInstalled checks if the launcher binary exists for the given GPU type.
func IsLauncherInstalled(gpuType string) bool {
	binary := DetectLauncherBinary(gpuType)
	if binary == "" {
		return false
	}
	path := filepath.Join(os.Getenv("HOME"), ".local", "bin", binary)
	_, err := os.Stat(path)
	return err == nil
}

// LauncherBinaryPath returns the full path to the launcher binary for the given GPU type.
func LauncherBinaryPath(gpuType string) string {
	binary := DetectLauncherBinary(gpuType)
	if binary == "" {
		return ""
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "bin", binary)
}

// LauncherDesktopPath returns the full path to the .desktop file for the given GPU type.
func LauncherDesktopPath(gpuType string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	binary := DetectLauncherBinary(gpuType)
	if binary == "" {
		return ""
	}
	return filepath.Join(homeDir, ".local", "share", "applications", binary+".desktop")
}

// LauncherIconPath returns the full path to the launcher icon.
func LauncherIconPath(gpuType string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png")
}

// LaunchVarsEnvPath returns the path to the launch_vars.env file in the WINEPREFIX.
func LaunchVarsEnvPath(wineprefix string) string {
	return filepath.Join(wineprefix, "launch_vars.env")
}

// LauncherExePath returns the path to the AstarteLauncher.exe inside the WINEPREFIX.
func LauncherExePath(wineprefix string) string {
	return filepath.Join(wineprefix, "drive_c/users/steamuser/AppData/Local/Astarte Industries/Astarte Launcher/AstarteLauncher.exe")
}

// LauncherLogPath returns the path to the launcher.log file in the WINEPREFIX.
func LauncherLogPath(wineprefix string) string {
	return filepath.Join(wineprefix, "launcher.log")
}

// EnsureWineBinaries checks that wine, wineboot, and wineserver are available.
// Returns the resolved binary paths.
func EnsureWineBinaries() (wine, wineboot, wineserver string, err error) {
	wine = core.LookPath("wine")
	if wine == "" {
		return "", "", "", fmt.Errorf("wine not found")
	}
	wineboot = core.LookPath("wineboot")
	if wineboot == "" {
		return "", "", "", fmt.Errorf("wineboot not found")
	}
	wineserver = core.LookPath("wineserver")
	if wineserver == "" {
		return "", "", "", fmt.Errorf("wineserver not found")
	}
	return wine, wineboot, wineserver, nil
}

// EnsureProtonBinary checks that the proton binary exists and is executable.
func EnsureProtonBinary(protonpath string) error {
	protonBin := filepath.Join(protonpath, "proton")
	if _, err := os.Stat(protonBin); err != nil {
		return fmt.Errorf("proton binary not found at %s: %w", protonBin, err)
	}
	if err := os.Chmod(protonBin, 0755); err != nil {
		return fmt.Errorf("failed to make proton binary executable: %w", err)
	}
	return nil
}

// GetLauncherScript returns the shared umu wrapper for compatibility.
func GetLauncherScript(wineprefix, protonpath string) (string, error) {
	return generateWrapperContent(LauncherConfig{Wineprefix: wineprefix, Protonpath: protonpath}), nil
}

// GetProtonLauncherScript returns the shared umu wrapper for compatibility.
func GetProtonLauncherScript(wineprefix, protonpath string) (string, error) {
	return GetLauncherScript(wineprefix, protonpath)
}

// ValidateLauncherEnvironment checks that all required files exist for the launcher to work.
func ValidateLauncherEnvironment(wineprefix string) error {
	// Check launch_vars.env
	if _, err := os.Stat(filepath.Join(wineprefix, "launch_vars.env")); err != nil {
		return fmt.Errorf("launch variables file not found: %w", err)
	}

	// Check launcher executable
	launcherExe := LauncherExePath(wineprefix)
	if _, err := os.Stat(launcherExe); err != nil {
		return fmt.Errorf("launcher executable not found: %s: %w", launcherExe, err)
	}

	// Check WINEPREFIX exists
	if _, err := os.Stat(wineprefix); err != nil {
		return fmt.Errorf("WINEPREFIX not found: %s: %w", wineprefix, err)
	}

	return nil
}

// GetDesktopFileTemplate returns the .desktop file content for the given GPU type.
func GetDesktopFileTemplate(entryName, entryExec, entryComment, installedIcon, wineprefix string) string {
	return fmt.Sprintf(`[Desktop Entry]
Name=%s
Comment=%s
Exec=%s
Icon=%s
Type=Application
Categories=Game;
Terminal=false
Path=%s
`, entryName, entryComment, entryExec, installedIcon, wineprefix)
}

// IsGioAvailable checks if the gio command is available.
func IsGioAvailable() bool {
	return core.LookPath("gio") != ""
}

// IsUpdateDesktopDatabaseAvailable checks if update-desktop-database is available.
func IsUpdateDesktopDatabaseAvailable() bool {
	return core.LookPath("update-desktop-database") != ""
}

// MarkDesktopTrusted marks a .desktop file as trusted using gio.
func MarkDesktopTrusted(desktopFile string) error {
	if !IsGioAvailable() {
		return nil
	}
	return runGioSet(desktopFile, "metadata::trusted", "true")
}

// UpdateDesktopDatabase updates the desktop database for the given directory.
func UpdateDesktopDatabase(dir string) error {
	if !IsUpdateDesktopDatabaseAvailable() {
		return nil
	}
	return runUpdateDesktopDatabase(dir)
}

// EnsureLauncherDirectories creates all necessary directories for the launcher.
func EnsureLauncherDirectories() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dirs := []string{
		filepath.Join(homeDir, ".local", "bin"),
		filepath.Join(homeDir, ".local", "share", "applications"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// EnsureUserLauncherDirectories creates user-level directories for icons and desktop files.
func EnsureUserLauncherDirectories() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dirs := []string{
		filepath.Join(homeDir, ".local", "share", "applications"),
		filepath.Join(homeDir, ".local", "share", "icons", "hicolor", "256x256", "apps"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// RemoveLauncherDesktop removes the .desktop file.
// All GPU types produce Bellum.desktop.
func RemoveLauncherDesktop(gpuType string) error {
	desktopPath := LauncherDesktopPath(gpuType)
	if desktopPath == "" {
		return nil
	}
	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove desktop file %s: %w", desktopPath, err)
	}

	// Also remove from ~/Desktop/ if it exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil // non-fatal
	}
	desktopDest := filepath.Join(homeDir, "Desktop", "Bellum.desktop")
	if err := os.Remove(desktopDest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove desktop shortcut %s: %w", desktopDest, err)
	}

	return nil
}

// RemoveLauncherIcon removes the launcher icon.
func RemoveLauncherIcon() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil // non-fatal
	}
	iconPath := filepath.Join(homeDir, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png")
	if err := os.Remove(iconPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove icon %s: %w", iconPath, err)
	}
	return nil
}

// RemoveLauncherBinary removes the launcher binary for the given GPU type.
func RemoveLauncherBinary(gpuType string) error {
	binaryPath := LauncherBinaryPath(gpuType)
	if binaryPath == "" {
		return nil
	}
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove launcher binary %s: %w", binaryPath, err)
	}
	return nil
}

// EnsureLauncherWrapper installs the shared umu wrapper in the user bin directory.
func EnsureLauncherWrapper(gpuType, wineprefix, protonpath string) error {
	scriptContent, err := GetLauncherScript(wineprefix, protonpath)
	if err != nil {
		return fmt.Errorf("failed to generate launcher script: %w", err)
	}

	scriptPath := filepath.Join(os.Getenv("HOME"), ".local", "bin", "Bellum")
	if err := writeWrapper(scriptPath, scriptContent); err != nil {
		return fmt.Errorf("failed to write launcher script to %s: %w", scriptPath, err)
	}

	return nil
}

// EnsureLauncherDesktop creates the .desktop file for the given GPU type.
// Both GPU types use "Bellum" as the binary and desktop name.
func EnsureLauncherDesktop(gpuType, wineprefix, iconPath string) error {
	if err := EnsureUserLauncherDirectories(); err != nil {
		return err
	}

	entryName := "Bellum"
	entryExec := filepath.Join(os.Getenv("HOME"), ".local", "bin", "Bellum")
	entryComment := "Launch Bellum via Proton and umu"

	installedIcon := iconPath
	if installedIcon == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			installedIcon = filepath.Join(homeDir, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png")
		}
	}

	desktopContent := GetDesktopFileTemplate(entryName, entryExec, entryComment, installedIcon, wineprefix)
	desktopPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications", entryName+".desktop")

	if err := os.WriteFile(desktopPath, []byte(desktopContent), 0644); err != nil {
		return fmt.Errorf("failed to write desktop file: %w", err)
	}

	// Copy to ~/Desktop/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil // non-fatal: home dir unavailable
	}
	desktopDest := filepath.Join(homeDir, "Desktop", entryName+".desktop")
	if _, err := os.Stat(filepath.Join(homeDir, "Desktop")); err == nil {
		_ = copyFile(desktopPath, desktopDest)
	}

	// Mark as trusted
	if IsGioAvailable() {
		_ = MarkDesktopTrusted(desktopPath)
		if _, err := os.Stat(desktopDest); err == nil {
			_ = MarkDesktopTrusted(desktopDest)
		}
	}

	// Update desktop database
	if IsUpdateDesktopDatabaseAvailable() {
		_ = UpdateDesktopDatabase(filepath.Join(homeDir, ".local", "share", "applications"))
	}

	return nil
}

// EnsureLauncherIcon copies the launcher icon to the user's icon directory.
func EnsureLauncherIcon(iconPath string) error {
	if iconPath == "" {
		return nil // non-fatal: icon is optional
	}
	return CopyIcon(iconPath)
}

// IsLauncherFullyInstalled checks if all launcher components exist for the given GPU type.
func IsLauncherFullyInstalled(gpuType, wineprefix string) bool {
	// Check binary
	if !IsLauncherInstalled(gpuType) {
		return false
	}

	// Check desktop file
	desktopPath := LauncherDesktopPath(gpuType)
	if _, err := os.Stat(desktopPath); err != nil {
		return false
	}

	// Check launch_vars.env
	if _, err := os.Stat(filepath.Join(wineprefix, "launch_vars.env")); err != nil {
		return false
	}

	// Check launcher executable
	if _, err := os.Stat(LauncherExePath(wineprefix)); err != nil {
		return false
	}

	return true
}

// DetectLauncherType reports whether the shared umu wrapper is installed.
func DetectLauncherType() string {
	bellumPath := filepath.Join(os.Getenv("HOME"), ".local", "bin", "Bellum")
	if exists, _ := fileExists(bellumPath); !exists {
		return ""
	}
	content, err := os.ReadFile(bellumPath)
	if err != nil {
		return ""
	}
	if strings.Contains(string(content), "umu-run") {
		return "Proton/umu"
	}
	return ""
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
