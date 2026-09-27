package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bellum-installer/pkg/core"
	"bellum-installer/pkg/gui"
)

// UninstallConfig holds configuration for uninstallation
type UninstallConfig struct {
	WINEPREFIX string
	GPUType    string
	DryRun     bool
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
	prefixExists := isDirWith(config.WINEPREFIX, boundaries.Files)
	if prefixExists {
		if err := validateBellumPrefix(config.WINEPREFIX); err != nil {
			return err
		}
		if _, err := readManifest(config.WINEPREFIX, boundaries.Files); err != nil {
			return err
		}
		logger.Info("[OK] Verified a Bellum install: its ownership manifest names this exact folder")
	} else if err := validateMissingBellumPrefix(config.WINEPREFIX); err != nil {
		return err
	}

	// Check if WINEPREFIX exists
	wineprefixExists := isDirWith(config.WINEPREFIX, boundaries.Files)
	if !wineprefixExists {
		logger.Warn(fmt.Sprintf("WINEPREFIX directory not found: %s", core.Colorize(config.WINEPREFIX, core.ColorBoldYellow)))
	}

	logger.Info(fmt.Sprintf("WINEPREFIX: %s", core.Colorize(config.WINEPREFIX, core.ColorBoldYellow)))
	logger.Info("Owned resources: the selected Bellum prefix and launcher assets that point to it. Shared Proton is retained.")
	if config.DryRun {
		logger.Info(fmt.Sprintf("Dry run: would remove only %s", config.WINEPREFIX))
		return nil
	}

	// Ask for confirmation before removing anything
	fmt.Println()
	if !core.AskBoolDefaultNo(fmt.Sprintf("Delete Bellum prefix %q and launcher assets that point to it? This cannot be undone. (y/N): ", config.WINEPREFIX)) {
		logger.Info("Uninstallation cancelled by user.")
		return nil
	}

	fmt.Println()
	logger.Info("Proceeding with uninstallation...")
	fmt.Println()

	// Remove WINEPREFIX if it exists
	if wineprefixExists {
		if err := removeWINEPREFIXWith(config.WINEPREFIX, logger, boundaries.Files); err != nil {
			return err
		}
	} else {
		logger.Info(fmt.Sprintf("Skipping WINEPREFIX removal: %s does not exist", core.Colorize(config.WINEPREFIX, core.ColorBoldYellow)))
	}
	if err := removeOwnedLauncherAssets(config.WINEPREFIX, logger, boundaries.Files, boundaries.Commands); err != nil {
		return err
	}

	logger.Info("[OK] Uninstallation complete!")
	fmt.Println()

	return nil
}

func validateMissingBellumPrefix(prefix string) error {
	clean, err := checkPrefixPath(prefix)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(clean); err == nil {
		return fmt.Errorf("prefix exists but is not a directory: %q", clean)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func removeOwnedLauncherAssets(prefix string, logger *core.Logger, files FileStore, commands CommandRunner) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	ownedDesktop := false
	for _, path := range []string{filepath.Join(home, ".local", "share", "applications", "Bellum.desktop"), filepath.Join(home, "Desktop", "Bellum.desktop")} {
		data, readErr := files.ReadFile(path)
		if readErr == nil && desktopPointsAt(string(data), prefix) {
			ownedDesktop = true
			if err := files.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove owned desktop entry: %w", err)
			}
			logger.Info(fmt.Sprintf("[OK] Removed %s", path))
		}
	}
	launcher := filepath.Join(home, ".local", "bin", "Bellum")
	if data, readErr := files.ReadFile(launcher); readErr == nil && strings.Contains(string(data), prefix) {
		if err := files.Remove(launcher); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove owned launcher: %w", err)
		}
		logger.Info(fmt.Sprintf("[OK] Removed %s", launcher))
	}
	if ownedDesktop {
		icon := filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png")
		if err := files.Remove(icon); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove Bellum icon: %w", err)
		}
		apps := filepath.Join(home, ".local", "share", "applications")
		if commands != nil {
			_ = commands.Run(core.RunModeSilent, []string{"update-desktop-database", apps}, logger, "")
		}
	}
	return nil
}

func desktopPointsAt(content, prefix string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(strings.TrimPrefix(line, "Path=")) == prefix && strings.HasPrefix(line, "Path=") {
			return true
		}
	}
	return false
}

// removeWINEPREFIX removes the WINEPREFIX directory with user confirmation
func removeWINEPREFIX(wineprefix string, logger *core.Logger) error {
	return removeWINEPREFIXWith(wineprefix, logger, DefaultBoundaries.Files)
}
func removeWINEPREFIXWith(wineprefix string, logger *core.Logger, files FileStore) error {
	if err := removeBellumPrefix(wineprefix, proofInstalled, files); err != nil {
		logger.Error(fmt.Sprintf("Failed to remove WINEPREFIX: %v", err))
		return err
	}

	logger.Info(fmt.Sprintf("[OK] Removed WINEPREFIX: %s", core.Colorize(wineprefix, core.ColorBoldYellow)))
	return nil
}

// validateBellumPrefix proves that an existing prefix is a Bellum install
// this user owns (see verifyBellumPrefix). It changes nothing.
func validateBellumPrefix(prefix string) error {
	_, err := verifyBellumPrefix(prefix, proofInstalled, DefaultBoundaries.Files)
	return err
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

	if err := validateBellumPrefix(selectedPath); err != nil {
		return "", err
	}

	logger.Info(fmt.Sprintf("[OK] WINEPREFIX found at %s", core.Colorize(selectedPath, core.ColorBoldYellow)))
	fmt.Println()

	return selectedPath, nil
}
