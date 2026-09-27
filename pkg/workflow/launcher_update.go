package workflow

import (
	"fmt"

	"bellum-installer/pkg/core"
	"bellum-installer/pkg/launchers"
	"bellum-installer/pkg/packages"
)

// UpdateLauncherInPrefix installs the latest Astarte Launcher release into a
// Bellum prefix from Linux (see packages.UpdateLauncher). The Bellum wrapper
// runs it before each launch through `bellum-installer update-launcher`.
func UpdateLauncherInPrefix(prefix string, logger *core.Logger) error {
	clean, err := checkPrefixPath(prefix)
	if err != nil {
		return err
	}
	if _, err := readManifest(clean, DefaultBoundaries.Files); err != nil {
		return fmt.Errorf("%s is not a Bellum install: %w", clean, err)
	}
	version, replaced, err := packages.UpdateLauncher(launchers.LauncherExePath(clean), packages.LauncherStampPath(clean), logger)
	if err != nil {
		return err
	}
	if replaced {
		logger.Info(fmt.Sprintf("[OK] Astarte Launcher updated to %s", version))
	} else {
		logger.Info(fmt.Sprintf("Astarte Launcher %s is up to date", version))
	}
	return nil
}

// launcherUpdater is replaced in tests so they never reach Astarte's servers.
var launcherUpdater = UpdateLauncherInPrefix

// updateLauncherStep runs UpdateLauncherInPrefix during install or update. A
// failure only warns: the wrapper tries again before every launch.
func updateLauncherStep(prefix string, logger *core.Logger) {
	if err := launcherUpdater(prefix, logger); err != nil {
		logger.Warn(fmt.Sprintf("Couldn't update the Astarte Launcher now (%v). Bellum tries again each time you start it.", err))
	}
}
