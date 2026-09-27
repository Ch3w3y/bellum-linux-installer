package workflow

import (
	"fmt"
	"os"
	"path/filepath"

	"bellum-installer/pkg/core"
	"bellum-installer/pkg/launchers"
	"bellum-installer/pkg/packages"
)

var requiredWinetricksVerbs = []string{"vcrun2022", "d3dcompiler_43", "d3dcompiler_47", "faudio", "msls31", "dotnet9", "dotnetdesktop9", "mfc140"}

// InstallConfig holds configuration for installation
type InstallConfig struct {
	WINEPREFIX        string
	ProtonPath        string
	GPUType           string
	GPUCapabilities   core.GPUCapabilities
	IsAMDGPU          bool
	LauncherInstaller string
	Workdir           string
	IsFSR41           bool
	// ReplaceIncomplete asks the installer to remove an unfinished earlier
	// install at WINEPREFIX (manifest plus install-incomplete marker) first.
	ReplaceIncomplete bool
}

// RunInstaller runs the main installation workflow
func RunInstaller(config InstallConfig, logger *core.Logger) error {
	return RunInstallerWithBoundaries(config, logger, DefaultBoundaries)
}

// RunInstallerWithBoundaries runs installation using explicit effect boundaries.
func RunInstallerWithBoundaries(config InstallConfig, logger *core.Logger, boundaries WorkflowBoundaries) (resultErr error) {
	if boundaries.Commands == nil {
		boundaries.Commands = DefaultBoundaries.Commands
	}
	if boundaries.AcquirePackage == nil {
		boundaries.AcquirePackage = DefaultBoundaries.AcquirePackage
	}
	if boundaries.GenerateLauncher == nil {
		boundaries.GenerateLauncher = DefaultBoundaries.GenerateLauncher
	}
	if boundaries.MutatePrefix == nil {
		boundaries.MutatePrefix = boundaries.Commands.Run
	}
	if boundaries.Files == nil {
		boundaries.Files = DefaultBoundaries.Files
	}
	logger.Info("Starting Installation")
	fmt.Println()

	setPrefixEnv(config.WINEPREFIX, config.ProtonPath)

	// Get launcher installer path
	launcherInstaller := config.LauncherInstaller
	if launcherInstaller == "" {
		state, err := boundaries.AcquirePackage(config.Workdir, logger)
		if err != nil {
			logger.Error("Failed to download launcher installer")
			return err
		}
		launcherInstaller = state.InstallerPath
		defer packages.CleanupLauncherInstaller(state, logger)
	}

	// Prechecks never create the prefix. Replace an unfinished earlier install
	// only now, after the user confirmed, and refuse anything else that is not
	// empty. Track whether this run created the directory so a failed install
	// removes only its own.
	if config.ReplaceIncomplete {
		logger.Info("Removing the unfinished previous install...")
		if err := discardIncompleteInstallWith(config.WINEPREFIX, boundaries.Files); err != nil {
			return err
		}
	}
	state, err := inspectPrefix(config.WINEPREFIX, boundaries.Files)
	if err != nil {
		return fmt.Errorf("inspect Wine prefix: %w", err)
	}
	if state != prefixAbsent && state != prefixEmpty {
		return fmt.Errorf("Wine prefix %s is not empty; refusing to install over it", config.WINEPREFIX)
	}
	createdPrefix := state == prefixAbsent
	defer func() {
		if resultErr != nil && createdPrefix {
			if err := rollbackNewPrefix(config.WINEPREFIX, true, boundaries.Files); err != nil {
				resultErr = fmt.Errorf("%w (also failed to roll back new prefix: %v)", resultErr, err)
			}
		}
	}()

	// Initialize WINEPREFIX with Proton base
	logger.Info("Initializing WINEPREFIX with Proton base")
	if err := os.MkdirAll(config.WINEPREFIX, 0700); err != nil {
		return fmt.Errorf("create private Wine prefix: %w", err)
	}
	if err := os.Chmod(config.WINEPREFIX, 0700); err != nil {
		return fmt.Errorf("secure Wine prefix: %w", err)
	}
	if err := writeManifest(config.WINEPREFIX, boundaries.Files); err != nil {
		return fmt.Errorf("write Bellum ownership manifest: %w", err)
	}
	if err := writeIncompleteMarker(config.WINEPREFIX, boundaries.Files); err != nil {
		return fmt.Errorf("mark install as in progress: %w", err)
	}
	logFile := filepath.Join(config.Workdir, "logs", "installer.log")
	// Every prefix operation goes through umu-run and the pinned Proton, so a
	// single Wine build owns the prefix. umu-run waits for the Wine server to
	// exit, so no separate wineserver -k is needed between steps. winetricks
	// is the copy shipped in Proton's protonfixes, pinned by the archive hash.
	// step runs one prefix command behind a spinner.
	step := func(label string, args []string) error {
		task := core.StartTask(label)
		err := boundaries.MutatePrefix(core.RunModeSilent, args, logger, logFile)
		task.Done(err)
		return err
	}

	if err := step("Creating the Wine prefix with Proton", umuRun("wineboot", "--init")); err != nil {
		logger.Error("Proton prefix initialisation (wineboot --init) failed")
		return err
	}

	for _, verb := range requiredWinetricksVerbs {
		if err := step("Installing "+winetricksLabel(verb), umuRun("winetricks", "-q", verb)); err != nil {
			logger.Error(fmt.Sprintf("Failed to install %s", verb))
			return err
		}
	}

	fmt.Println()
	logger.Info("The Astarte Launcher installer opens next. Follow its prompts; this window waits for it.")
	// Run the launcher installer the same way the Bellum wrapper runs the game.
	if err := step("Running the Astarte Launcher installer", umuRun(launcherInstaller)); err != nil {
		logger.Error("Launcher installation failed.")
		return err
	}
	if err := checkWebView2Runtime(config.WINEPREFIX); err != nil {
		return err
	}
	logger.Warn("Almost done. Don't start the game or close this window yet.")

	if err := step("Setting Windows 11 mode", umuRun("winetricks", "-q", "win11")); err != nil {
		logger.Warn("winetricks win11 failed (may be expected)")
	}

	// Proton's pinned build supplies DXVK, vkd3d-proton, and dxvk-nvapi.
	// Do not overlay a separate DXVK build into the prefix: retain one coherent
	// runtime set and its upstream integrity guarantees for every GPU vendor.
	if err := step("Tuning window and input settings", umuRun("winetricks", "-q", "grabfullscreen=y", "windowmanagerdecorated=n", "mwo=disable")); err != nil {
		logger.Error("Winetricks configuration failed.")
		return err
	}

	if config.IsAMDGPU {
		if err := step("Removing Wine Mono (AMD)", umuRun("winetricks", "-q", "remove_mono")); err != nil {
			logger.Error("Mono removal failed.")
			return err
		}
	}

	// Generate launcher
	if err := generateLauncherWith(config, logger, boundaries.GenerateLauncher); err != nil {
		return err
	}

	// Set DLL overrides
	boundaries.MutatePrefix(core.RunModeSilent, umuRun("reg", "add", `HKCU\Software\Wine\DirectInput`, "/v", "RawInput", "/t", "REG_DWORD", "/d", "1", "/f"), logger, logFile)

	return nil
}

// winetricksLabel names a winetricks verb for the progress display.
func winetricksLabel(verb string) string {
	names := map[string]string{
		"vcrun2022":      "Visual C++ 2015-2022 runtime",
		"d3dcompiler_43": "D3D compiler 43",
		"d3dcompiler_47": "D3D compiler 47",
		"faudio":         "FAudio",
		"msls31":         "Microsoft Line Services",
		"dotnet9":        ".NET 9 runtime",
		"dotnetdesktop9": ".NET 9 desktop runtime",
		"mfc140":         "MFC 14 runtime",
	}
	if n, ok := names[verb]; ok {
		return n
	}
	return verb
}

// setPrefixEnv points umu-run and Proton at the Bellum prefix for the
// commands this process runs.
func setPrefixEnv(prefix, protonPath string) {
	os.Setenv("PROTONPATH", protonPath)
	os.Setenv("WINEPREFIX", prefix)
	os.Setenv("WINEARCH", "win64")
	os.Setenv("STEAM_APP_PATH", prefix)
	os.Setenv("STEAM_APPID", "1")
	os.Setenv("STEAM_COMPAT_DATA_PATH", prefix)
	os.Setenv("STEAM_COMPAT_CLIENT_INSTALL_PATH", filepath.Join(os.Getenv("HOME"), ".steam", "steam"))
	os.Setenv("GAMEID", "1")
}

// RunUpdate moves a finished install onto the current pins: it refreshes the
// launcher wrapper and desktop entry and rewrites launch_vars.env and the
// registry overrides (through RunConfiguration afterwards). The prefix, the
// game and the launcher login are left alone. Proton upgrades the prefix
// itself on the next launch.
func RunUpdate(config InstallConfig, logger *core.Logger) error {
	return runUpdateWith(config, logger, DefaultBoundaries.GenerateLauncher)
}

func runUpdateWith(config InstallConfig, logger *core.Logger, generate func(launchers.LauncherConfig) error) error {
	state, err := inspectPrefix(config.WINEPREFIX, DefaultBoundaries.Files)
	if err != nil {
		return err
	}
	if state != prefixInstalled {
		return fmt.Errorf("%s is not a finished Bellum install", config.WINEPREFIX)
	}
	logger.Info("Updating the Bellum launcher and settings...")
	setPrefixEnv(config.WINEPREFIX, config.ProtonPath)
	return generateLauncherWith(config, logger, generate)
}

// umuRunBinary is the pinned umu-run installed by AcquireRuntime. It
// defaults to "umu-run" on PATH so tests and older callers still work.
var umuRunBinary = "umu-run"

// umuRun builds a command that runs inside the prefix through umu-run and the
// Proton selected by PROTONPATH.
func umuRun(args ...string) []string {
	return append([]string{umuRunBinary}, args...)
}

// checkWebView2Runtime rejects a bootstrapper-only installation. The launcher
// needs the installed runtime to keep its authentication UI alive.
func checkWebView2Runtime(prefix string) error {
	for _, programFiles := range []string{"Program Files (x86)", "Program Files"} {
		pattern := filepath.Join(prefix, "drive_c", programFiles, "Microsoft", "EdgeWebView", "Application", "*", "msedgewebview2.exe")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && info.Mode().IsRegular() {
				return nil
			}
		}
	}
	return fmt.Errorf("WebView2 runtime missing from Wine prefix %q: install msedgewebview2.exe before launching Bellum", prefix)
}

// GenerateLauncher generates the launcher wrappers and desktop files
func GenerateLauncher(config InstallConfig, logger *core.Logger) error {
	return generateLauncherWith(config, logger, launchers.GenerateLauncher)
}

func generateLauncherWith(config InstallConfig, logger *core.Logger, generate func(launchers.LauncherConfig) error) error {
	iconPath := filepath.Join(config.Workdir, "packages", "launcher_1_256x256x32.png")
	if err := packages.VerifySHA256(iconPath, packages.IconSHA256); err != nil {
		return err
	}
	// Generate launcher config
	launcherConfig := launchers.LauncherConfig{
		Wineprefix: config.WINEPREFIX,
		Protonpath: config.ProtonPath,
		GPUType:    config.GPUType,
		IconPath:   iconPath,
	}

	assets, err := snapshotLauncherAssets()
	if err != nil {
		return err
	}
	if err := generate(launcherConfig); err != nil {
		if restoreErr := assets.restore(); restoreErr != nil {
			return fmt.Errorf("%w (also failed to restore previous launcher assets: %v)", err, restoreErr)
		}
		logger.Error(fmt.Sprintf("Failed to generate launcher: %v", err))
		return err
	}

	logger.Info("[OK] Game launcher installed in ~/.local/bin/Bellum")

	return nil
}

type launcherAsset struct {
	path   string
	data   []byte
	mode   os.FileMode
	exists bool
}

type launcherAssets []launcherAsset

func snapshotLauncherAssets() (launcherAssets, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	paths := []string{
		filepath.Join(home, ".local", "bin", "Bellum"),
		filepath.Join(home, ".local", "share", "applications", "Bellum.desktop"),
		filepath.Join(home, "Desktop", "Bellum.desktop"),
		filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png"),
	}
	assets := make([]launcherAsset, 0, len(paths))
	for _, path := range paths {
		info, statErr := os.Lstat(path)
		if os.IsNotExist(statErr) {
			assets = append(assets, launcherAsset{path: path})
			continue
		}
		if statErr != nil {
			return nil, statErr
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("launcher asset is not a regular file: %s", path)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		assets = append(assets, launcherAsset{path: path, data: data, mode: info.Mode().Perm(), exists: true})
	}
	return assets, nil
}

func (assets launcherAssets) restore() error {
	var first error
	for _, asset := range assets {
		var err error
		if asset.exists {
			if err = os.MkdirAll(filepath.Dir(asset.path), 0755); err == nil {
				err = os.WriteFile(asset.path, asset.data, asset.mode)
			}
		} else {
			err = os.Remove(asset.path)
			if os.IsNotExist(err) {
				err = nil
			}
		}
		if err != nil && first == nil {
			first = err
		}
	}
	return first
}
