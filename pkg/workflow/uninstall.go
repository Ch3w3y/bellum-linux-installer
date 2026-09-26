package workflow

import (
	"fmt"
	"os"
	"path/filepath"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
	"bellum-installer/pkg/gui"
	"bellum-installer/pkg/packages"
)

// UninstallConfig holds configuration for uninstallation
type UninstallConfig struct {
	WINEPREFIX string
	GPUType    string
}

// RunUninstallation runs the uninstallation workflow
func RunUninstallation(config UninstallConfig, logger *core.Logger) error {
	return RunUninstallationWithBoundaries(config, logger, DefaultBoundaries)
}

// RunUninstallationWithBoundaries exposes host effects for safe, fake-backed tests.
func RunUninstallationWithBoundaries(config UninstallConfig, logger *core.Logger, boundaries WorkflowBoundaries) error {
	if boundaries.Commands == nil {
		boundaries.Commands = DefaultBoundaries.Commands
	}
	if boundaries.Files == nil {
		boundaries.Files = DefaultBoundaries.Files
	}

	logger.Info(fmt.Sprintf("GPU Type: %s", config.GPUType))
	logger.Info("Starting uninstallation phase...")
	fmt.Println()

	// Validate WINEPREFIX
	if config.WINEPREFIX == "" {
		logger.Error("WINEPREFIX is required. Use --wineprefix <path> or set WINEPREFIX environment variable.")
		return fmt.Errorf("WINEPREFIX is required")
	}
	if err := validateBellumPrefix(config.WINEPREFIX); err != nil {
		return err
	}

	// Check if WINEPREFIX exists
	wineprefixExists := isDirWith(config.WINEPREFIX, boundaries.Files)
	if !wineprefixExists {
		logger.Warn(fmt.Sprintf("WINEPREFIX directory not found: %s", core.Colorize(config.WINEPREFIX, core.ColorBoldYellow)))
	}

	logger.Info(fmt.Sprintf("WINEPREFIX: %s", core.Colorize(config.WINEPREFIX, core.ColorBoldYellow)))

	// Ask for confirmation before removing anything
	fmt.Println()
	if !core.AskBoolDefaultNo(fmt.Sprintf("Delete Bellum prefix %q and its launcher files? This cannot be undone. (y/N): ", config.WINEPREFIX)) {
		logger.Info("Uninstallation cancelled by user.")
		return nil
	}

	fmt.Println()
	logger.Info("Proceeding with uninstallation...")
	fmt.Println()

	// Remove launcher binaries
	if err := removeLauncherBinariesWith(config.GPUType, logger, boundaries.Files); err != nil {
		return err
	}

	// Remove desktop entries
	if err := removeDesktopEntriesWith(config.GPUType, logger, boundaries.Files, boundaries.Commands); err != nil {
		return err
	}

	// Remove icon
	if err := removeIconWith(logger, boundaries.Files); err != nil {
		return err
	}

	// Remove Proton directory
	if err := removeProtonWith(config.WINEPREFIX, config.GPUType, logger, boundaries.Files); err != nil {
		return err
	}

	// Remove WINEPREFIX if it exists
	if wineprefixExists {
		if err := removeWINEPREFIXWith(config.WINEPREFIX, logger, boundaries.Files); err != nil {
			return err
		}
	} else {
		logger.Info(fmt.Sprintf("Skipping WINEPREFIX removal: %s does not exist", core.Colorize(config.WINEPREFIX, core.ColorBoldYellow)))
	}

	logger.Info("[OK] Uninstallation complete!")
	fmt.Println()

	return nil
}

// removeLauncherBinaries removes the launcher wrapper script.
func removeLauncherBinaries(gpuType string, logger *core.Logger) error {
	return removeLauncherBinariesWith(gpuType, logger, DefaultBoundaries.Files)
}
func removeLauncherBinariesWith(gpuType string, logger *core.Logger, files FileStore) error {
	logger.Info("Removing launcher binaries...")

	bellumPath := filepath.Join(os.Getenv("HOME"), ".local", "bin", "Bellum")
	if _, err := files.Stat(bellumPath); err == nil {
		if err := files.Remove(bellumPath); err != nil {
			logger.Warn(fmt.Sprintf("Failed to remove %s: %v", bellumPath, err))
		} else {
			logger.Info(fmt.Sprintf("[OK] Removed %s", bellumPath))
		}
	}

	return nil
}

// removeDesktopEntries removes the .desktop files.
func removeDesktopEntries(gpuType string, logger *core.Logger) error {
	return removeDesktopEntriesWith(gpuType, logger, DefaultBoundaries.Files, DefaultBoundaries.Commands)
}
func removeDesktopEntriesWith(gpuType string, logger *core.Logger, files FileStore, commands CommandRunner) error {
	logger.Info("Removing desktop entries...")

	userAppsDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications")

	desktopPath := filepath.Join(userAppsDir, "Bellum.desktop")
	if _, err := files.Stat(desktopPath); err == nil {
		if err := files.Remove(desktopPath); err != nil {
			logger.Warn(fmt.Sprintf("Failed to remove %s: %v", desktopPath, err))
		} else {
			logger.Info(fmt.Sprintf("[OK] Removed %s", desktopPath))
		}
	}

	homeDir := os.Getenv("HOME")
	desktopDest := filepath.Join(homeDir, "Desktop", "Bellum.desktop")
	if _, err := files.Stat(desktopDest); err == nil {
		if err := files.Remove(desktopDest); err != nil {
			logger.Warn(fmt.Sprintf("Failed to remove %s: %v", desktopDest, err))
		} else {
			logger.Info(fmt.Sprintf("[OK] Removed %s", desktopDest))
		}
	}

	if _, err := files.Stat(userAppsDir); err == nil {
		commands.Run(core.RunModeSilent, []string{"update-desktop-database", userAppsDir}, logger, "")
	}

	return nil
}

// removeIcon removes the launcher icon from user-level icon directory
func removeIcon(logger *core.Logger) error {
	return removeIconWith(logger, DefaultBoundaries.Files)
}
func removeIconWith(logger *core.Logger, files FileStore) error {
	logger.Info("Removing launcher icon...")

	homeDir := os.Getenv("HOME")
	iconPath := filepath.Join(homeDir, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png")
	if _, err := files.Stat(iconPath); err == nil {
		if err := files.Remove(iconPath); err != nil {
			logger.Warn(fmt.Sprintf("Failed to remove %s: %v", iconPath, err))
		} else {
			logger.Info(fmt.Sprintf("[OK] Removed %s", iconPath))
		}
	}

	return nil
}

// removeProton removes the Proton directory
func removeProton(wineprefix string, gpuType string, logger *core.Logger) error {
	return removeProtonWith(wineprefix, gpuType, logger, DefaultBoundaries.Files)
}
func removeProtonWith(wineprefix string, gpuType string, logger *core.Logger, files FileStore) error {
	logger.Info("Removing Proton directory...")

	// Use proton-cachyos for all GPUs (AMD and NVIDIA)
	protonVer := config.DefaultVersions.ProtonVer

	// Get the proton install path (even if wineprefix doesn't exist)
	protonPath := packages.GetProtonInstallPath(protonVer)
	if _, err := files.Stat(protonPath); err == nil {
		logger.Info(fmt.Sprintf("Removing Proton directory: %s", protonPath))
		if err := files.RemoveAll(protonPath); err != nil {
			logger.Warn(fmt.Sprintf("Failed to remove Proton directory %s: %v", protonPath, err))
		} else {
			logger.Info("[OK] Removed Bellum Proton directory")
		}
	}

	// Check if the parent proton directory is now empty and remove it silently
	protonParentDir := filepath.Join(filepath.Dir(protonPath))
	if isEmptyDirWith(protonParentDir, files) {
		files.RemoveAll(protonParentDir)
	}

	// Check if the bellum directory is now empty and remove it silently
	bellumDir := filepath.Join(filepath.Dir(filepath.Dir(protonParentDir)))
	if isEmptyDirWith(bellumDir, files) {
		files.RemoveAll(bellumDir)
	}

	return nil
}

// isEmptyDir checks if a directory is empty
func isEmptyDir(path string) bool {
	return isEmptyDirWith(path, DefaultBoundaries.Files)
}

func isEmptyDirWith(path string, files FileStore) bool {
	entries, err := files.ReadDir(path)
	return err == nil && len(entries) == 0
}

// removeWINEPREFIX removes the WINEPREFIX directory with user confirmation
func removeWINEPREFIX(wineprefix string, logger *core.Logger) error {
	return removeWINEPREFIXWith(wineprefix, logger, DefaultBoundaries.Files)
}
func removeWINEPREFIXWith(wineprefix string, logger *core.Logger, files FileStore) error {
	if err := validateBellumPrefix(wineprefix); err != nil {
		return err
	}
	if err := files.RemoveAll(wineprefix); err != nil {
		logger.Error(fmt.Sprintf("Failed to remove WINEPREFIX: %v", err))
		return err
	}

	logger.Info(fmt.Sprintf("[OK] Removed WINEPREFIX: %s", core.Colorize(wineprefix, core.ColorBoldYellow)))
	return nil
}

func validateBellumPrefix(prefix string) error {
	if !filepath.IsAbs(prefix) {
		return fmt.Errorf("Bellum prefix must be absolute: %q", prefix)
	}
	clean := filepath.Clean(prefix)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if clean == "/" || clean == filepath.Clean(home) {
		return fmt.Errorf("refusing destructive prefix: %q", clean)
	}
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return fmt.Errorf("cannot resolve Bellum prefix %q: %w", clean, err)
	}
	if resolved == "/" || resolved == filepath.Clean(home) || resolved != clean {
		return fmt.Errorf("refusing symlinked or unsafe Bellum prefix: %q", clean)
	}
	if filepath.Base(clean) != "Bellum" {
		return fmt.Errorf("prefix lacks Bellum directory marker: %q", clean)
	}
	if _, err := os.Stat(filepath.Join(clean, "system.reg")); err != nil {
		return fmt.Errorf("prefix lacks Wine system.reg marker: %q", clean)
	}
	if info, err := os.Stat(filepath.Join(clean, "drive_c")); err != nil || !info.IsDir() {
		return fmt.Errorf("prefix lacks Wine drive_c marker: %q", clean)
	}
	return nil
}

// ValidateWINEPREFIXWithGUIForUninstall prompts user to select a WINEPREFIX using GUI picker
// This function handles the complete workflow of:
// 1. Opening GUI directory picker to select the Bellum WINEPREFIX directory directly
// 2. Validating the WINEPREFIX exists
// Returns the WINEPREFIX path (selected directory)
func ValidateWINEPREFIXWithGUIForUninstall(logger *core.Logger) (string, error) {
	// Open GUI directory picker for existing directory
	fmt.Println()
	logger.Info("Select the Bellum WINEPREFIX directory to uninstall...")
	logger.Info("This will uninstall Bellum from the selected location.")
	fmt.Println()

	result, err := gui.PickDirectoryExisting("")
	if err != nil {
		return "", fmt.Errorf("failed to pick directory: %w", err)
	}

	if !result.Success {
		return "", fmt.Errorf("directory selection cancelled or failed: %v", result.Error)
	}

	selectedPath := result.Path
	logger.Info(fmt.Sprintf("Selected directory: %s", core.Colorize(selectedPath, core.ColorBoldYellow)))
	fmt.Println()

	// Validate the WINEPREFIX exists
	if !isDir(selectedPath) {
		return "", fmt.Errorf("selected directory does not exist: %s", selectedPath)
	}

	// Verify it's a valid Bellum installation by checking for typical files
	entries, err := os.ReadDir(selectedPath)
	if err != nil || len(entries) == 0 {
		logger.Warn(fmt.Sprintf("WINEPREFIX directory %s appears to be empty or invalid", selectedPath))
		if !core.AskBool("Are you sure you want to proceed with uninstallation? (Y/n): ") {
			return "", fmt.Errorf("uninstallation cancelled by user")
		}
	}

	logger.Info(fmt.Sprintf("[OK] WINEPREFIX found at %s", core.Colorize(selectedPath, core.ColorBoldYellow)))
	fmt.Println()

	return selectedPath, nil
}
