package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cfg "bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
	"bellum-installer/pkg/launchers"
	"bellum-installer/pkg/packages"
)

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
}

// InstallDXVK installs DXVK for AMD GPUs
func InstallDXVK(gpuType string, workdir string, logger *core.Logger) error {
	return InstallDXVKWithBoundaries(gpuType, workdir, os.Getenv("WINEPREFIX"), logger, DefaultBoundaries)
}

// InstallDXVKWithBoundaries makes archive handling, host commands, and prefix
// file writes replaceable in tests and by callers embedding the workflow.
func InstallDXVKWithBoundaries(gpuType, workdir, wineprefix string, logger *core.Logger, boundaries WorkflowBoundaries) error {
	if boundaries.Files == nil {
		boundaries.Files = DefaultBoundaries.Files
	}
	if boundaries.Commands == nil {
		boundaries.Commands = DefaultBoundaries.Commands
	}
	if boundaries.VerifyFile == nil {
		boundaries.VerifyFile = DefaultBoundaries.VerifyFile
	}
	if boundaries.ExtractPackage == nil {
		boundaries.ExtractPackage = DefaultBoundaries.ExtractPackage
	}
	if boundaries.CleanupPackage == nil {
		boundaries.CleanupPackage = DefaultBoundaries.CleanupPackage
	}
	// Only install DXVK for AMD GPUs
	if !strings.Contains(strings.ToLower(gpuType), "amd") && !strings.Contains(strings.ToLower(gpuType), "radeon") {
		logger.Info(fmt.Sprintf("Skipping DXVK installation for non-AMD GPU: %s", gpuType))
		return nil
	}

	archive := filepath.Join(workdir, "packages", "dxvk-"+cfg.DefaultVersions.DXVKVer+".tar.gz")
	var tmpDir string

	logger.Info("Installing DXVK...")
	if _, err := boundaries.Files.Stat(archive); os.IsNotExist(err) {
		logger.Error(fmt.Sprintf("DXVK archive not found: %s", archive))
		return fmt.Errorf("DXVK archive not found: %s", archive)
	}
	if err := boundaries.VerifyFile(archive, packages.DXVKSHA256); err != nil {
		return err
	}

	tmpDir, err := boundaries.ExtractPackage(archive, "dxvk")
	if err != nil {
		logger.Error("Failed to extract DXVK archive")
		return err
	}

	// Find the dxvk_setup.sh script
	installDir := tmpDir
	if _, err := boundaries.Files.Stat(filepath.Join(tmpDir, "dxvk_setup.sh")); os.IsNotExist(err) {
		// Try to find subdirectory
		entries, err := boundaries.Files.ReadDir(tmpDir)
		if err != nil || len(entries) == 0 {
			logger.Error("DXVK setup script not found after extraction.")
			boundaries.CleanupPackage(archive)
			return fmt.Errorf("DXVK setup script not found")
		}
		for _, entry := range entries {
			if entry.IsDir() {
				installDir = filepath.Join(tmpDir, entry.Name())
				break
			}
		}
	}

	if _, err := boundaries.Files.Stat(filepath.Join(installDir, "dxvk_setup.sh")); os.IsNotExist(err) {
		logger.Error("DXVK setup script not found after extraction.")
		boundaries.CleanupPackage(archive)
		return fmt.Errorf("DXVK setup script not found")
	}

	// Run dxvk_setup.sh install
	logFile := filepath.Join(workdir, "logs", "installer.log")
	if err := boundaries.Commands.Run(core.RunModeSilent, []string{filepath.Join(installDir, "dxvk_setup.sh"), "install"}, logger, logFile); err != nil {
		logger.Error("DXVK installation failed.")
		boundaries.CleanupPackage(archive)
		return err
	}

	// Copy dxvk.conf to WINEPREFIX
	dxvkConf := filepath.Join(installDir, "dxvk.conf")
	if wineprefix == "" {
		logger.Error("WINEPREFIX not set")
		boundaries.CleanupPackage(archive)
		return fmt.Errorf("WINEPREFIX not set")
	}

	content, err := boundaries.Files.ReadFile(dxvkConf)
	if err == nil {
		err = boundaries.Files.WriteFile(filepath.Join(wineprefix, "dxvk.conf"), content, 0644)
	}
	if err != nil {
		logger.Error("Failed to copy dxvk.conf.")
		boundaries.CleanupPackage(archive)
		return err
	}

	boundaries.CleanupPackage(archive)
	logger.Info("[OK] DXVK installed")
	return nil
}

// RunInstaller runs the main installation workflow
func RunInstaller(config InstallConfig, logger *core.Logger) error {
	return RunInstallerWithBoundaries(config, logger, DefaultBoundaries)
}

// RunInstallerWithBoundaries runs installation using explicit effect boundaries.
func RunInstallerWithBoundaries(config InstallConfig, logger *core.Logger, boundaries WorkflowBoundaries) error {
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
	logger.Info("Starting Installation")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	winetricks := filepath.Join(homeDir, ".local", "bin", "winetricks")
	fmt.Println()

	// Set environment variables
	os.Setenv("PROTONPATH", config.ProtonPath)
	os.Setenv("WINEPREFIX", config.WINEPREFIX)
	os.Setenv("WINEARCH", "win64")
	os.Setenv("STEAM_APP_PATH", config.WINEPREFIX)
	os.Setenv("STEAM_APPID", "1")
	os.Setenv("STEAM_COMPAT_DATA_PATH", config.WINEPREFIX)
	os.Setenv("STEAM_COMPAT_CLIENT_INSTALL_PATH", filepath.Join(os.Getenv("HOME"), ".steam", "steam"))
	os.Setenv("GAMEID", "1")

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

	// Initialize WINEPREFIX with Proton base
	logger.Info("Initializing WINEPREFIX with Proton base")
	if err := os.MkdirAll(config.WINEPREFIX, 0700); err != nil {
		return fmt.Errorf("create private Wine prefix: %w", err)
	}
	if err := os.Chmod(config.WINEPREFIX, 0700); err != nil {
		return fmt.Errorf("secure Wine prefix: %w", err)
	}
	logFile := filepath.Join(config.Workdir, "logs", "installer.log")
	if err := boundaries.MutatePrefix(core.RunModeSilent, []string{"umu-run", cfg.DefaultVersions.Binaries.Msidb}, logger, logFile); err != nil {
		logger.Warn("umu-run /usr/bin/msidb failed")
		return err
	}

	if err := boundaries.MutatePrefix(core.RunModeSilent, []string{cfg.DefaultVersions.Binaries.Wineboot, "--init"}, logger, logFile); err != nil {
		logger.Error("wineboot --init failed")
		return err
	}

	// Install required winedlls
	logger.Info("Installing required winedlls")
	dlls := []string{
		"vcrun2026",
		"d3dcompiler_43",
		"d3dcompiler_47",
		"faudio",
		"msls31",
		"dotnet9",
		"dotnetdesktop9",
		"mfc140",
	}

	for _, dll := range dlls {
		if err := boundaries.MutatePrefix(core.RunModeSilent, []string{winetricks, "-q", dll}, logger, logFile); err != nil {
			logger.Error(fmt.Sprintf("Failed to install %s", dll))
			return err
		}
		logger.Info(fmt.Sprintf("[OK] %s", dll))
	}

	fmt.Println()
	logger.Info("Time to install the launcher! Follow the on screen prompts once the GUI pops up.")

	// Kill wine server before running installer
	boundaries.MutatePrefix(core.RunModeSilent, []string{"wineserver", "-k"}, logger, logFile)

	// Run the launcher installer
	proton := filepath.Join(config.ProtonPath, "proton")
	if err := boundaries.MutatePrefix(core.RunModeSilent, []string{proton, "run", launcherInstaller}, logger, logFile); err != nil {
		logger.Error("Launcher installation failed.")
		return err
	}
	if err := checkWebView2Runtime(config.WINEPREFIX); err != nil {
		return err
	}

	logger.Info("Astarte Launcher install completed successfully! Few more steps to go...")
	logger.Warn("I'm not done! Don't launch game or close this script just yet")

	// Set Windows 11
	if err := boundaries.MutatePrefix(core.RunModeSilent, []string{winetricks, "win11"}, logger, logFile); err != nil {
		logger.Warn("winetricks win11 failed (may be expected)")
	}

	fmt.Println()

	// Install DXVK (AMD only)
	if config.GPUCapabilities.Vendor == core.GPUAMD {
		if err := InstallDXVKWithBoundaries("AMD", config.Workdir, config.WINEPREFIX, logger, boundaries); err != nil {
			return err
		}
	}

	// Configure WINEPREFIX
	logger.Info("Configuring WINEPREFIX with things Bellum likes")
	if err := boundaries.MutatePrefix(core.RunModeSilent, []string{winetricks, "grabfullscreen=y", "windowmanagerdecorated=n", "mwo=disabled"}, logger, logFile); err != nil {
		logger.Error("Winetricks configuration failed.")
		return err
	}

	// Remove mono for AMD GPUs
	if config.IsAMDGPU {
		if err := boundaries.MutatePrefix(core.RunModeSilent, []string{winetricks, "remove_mono"}, logger, logFile); err != nil {
			logger.Error("Mono removal failed.")
			return err
		}
	}

	// Generate launcher
	if err := generateLauncherWith(config, logger, boundaries.GenerateLauncher); err != nil {
		return err
	}

	// Set DLL overrides
	boundaries.MutatePrefix(core.RunModeSilent, []string{"wine", "reg", "add", `HKCU\Software\Wine\DirectInput`, "/v", "RawInput", "/t", "REG_DWORD", "/d", "1", "/f"}, logger, logFile)

	// End wine session
	boundaries.MutatePrefix(core.RunModeSilent, []string{"wineboot", "--end-session"}, logger, logFile)

	return nil
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
	// Copy icon to system location
	iconPath := filepath.Join(config.Workdir, "packages", "launcher_1_256x256x32.png")
	if err := packages.VerifySHA256(iconPath, packages.IconSHA256); err != nil {
		return err
	}
	if err := launchers.CopyIcon(iconPath); err != nil {
		logger.Warn(fmt.Sprintf("Failed to copy icon: %v", err))
	}

	// Generate launcher config
	launcherConfig := launchers.LauncherConfig{
		Wineprefix: config.WINEPREFIX,
		Protonpath: config.ProtonPath,
		GPUType:    config.GPUType,
		IconPath:   iconPath,
	}

	if err := generate(launcherConfig); err != nil {
		logger.Error(fmt.Sprintf("Failed to generate launcher: %v", err))
		return err
	}

	logger.Info("[OK] Game launcher installed in ~/.local/bin/Bellum")

	return nil
}
