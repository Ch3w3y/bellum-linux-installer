// Package main provides the entry point for the Bellum installer.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
	"bellum-installer/pkg/workflow"
)

func main() {
	if err := core.RequireNonRoot(os.Geteuid()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Print banner
	printInstallerBanner()

	// Parse command line arguments
	forceWineVersion := flag.Bool("force-wine-version", false, "Force Wine version check")
	wineprefix := flag.String("wineprefix", "", "Path to WINEPREFIX directory (optional if WINEPREFIX env var is set)")
	launcherInstaller := flag.String("launcher-installer", "", "Path to launcher installer executable")
	help := flag.Bool("help", false, "Show help message")

	flag.Parse()

	if *help {
		fmt.Println("Bellum Linux Installer")
		fmt.Println()
		fmt.Println("Usage: bellum-installer [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --force-wine-version  Force Wine version check (not recommended)")
		fmt.Println("  --wineprefix PATH     Path to WINEPREFIX directory (optional if WINEPREFIX env var is set)")
		fmt.Println("  --launcher-installer PATH  Path to launcher installer executable")
		fmt.Println("  --help                Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  bellum-installer --wineprefix /path/to/wineprefix")
		fmt.Println("  bellum-installer --wineprefix /path/to/wineprefix --launcher-installer /path/to/launcher.exe")
		os.Exit(0)
	}

	// Determine workdir (directory containing the binary)
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to get executable path: %v\n", err)
		os.Exit(1)
	}
	workdir := filepath.Dir(exePath)

	// Create log directory and file
	logDir := filepath.Join(workdir, "logs")
	logFile := filepath.Join(logDir, "installer.log")

	// Initialize logger
	logger, err := core.NewLogger(logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Info("Bellum Linux Installer")

	// Determine WINEPREFIX
	var selectedWINEPREFIX string
	if *wineprefix != "" {
		selectedWINEPREFIX = *wineprefix
		logger.Info(fmt.Sprintf("WINEPREFIX from flag: %s", core.Colorize(selectedWINEPREFIX, core.ColorBoldYellow)))
	} else if envPrefix := os.Getenv("WINEPREFIX"); envPrefix != "" {
		selectedWINEPREFIX = envPrefix
		logger.Info(fmt.Sprintf("WINEPREFIX from environment: %s", core.Colorize(selectedWINEPREFIX, core.ColorBoldYellow)))
	} else {
		// Use GUI-based WINEPREFIX selection
		selectedWINEPREFIX, err = workflow.ValidateWINEPREFIXWithGUI(logger)
		if err != nil {
			logger.Error(fmt.Sprintf("WINEPREFIX selection failed: %v", err))
			os.Exit(1)
		}
	}

	// Resolve WINEPREFIX to absolute path if needed
	if !filepath.IsAbs(selectedWINEPREFIX) {
		absWINEPREFIX, err := filepath.Abs(selectedWINEPREFIX)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to resolve WINEPREFIX to absolute path: %v", err))
			os.Exit(1)
		}
		selectedWINEPREFIX = absWINEPREFIX
	}

	// Run prechecks with absolute paths
	result, err := workflow.RunPrechecks(selectedWINEPREFIX, *launcherInstaller, *forceWineVersion, false, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("Prechecks failed: %v", err))
		os.Exit(1)
	}

	// Print installer summary
	core.PrintInstallerSummary(
		result.ProtonVer,
		config.DefaultVersions.WineVer,
		config.DefaultVersions.WinetricksVer,
		config.DefaultVersions.VKD3DVer,
		config.DefaultVersions.DXVKVer,
		result.WINEPREFIX,
		result.LauncherInstaller,
		result.GPUType,
		workdir,
	)

	// Confirm with user before proceeding
	if !core.ConfirmProceed() {
		if result.LauncherTempDir != "" {
			_ = os.RemoveAll(result.LauncherTempDir)
		}
		fmt.Println("Installation cancelled.")
		os.Exit(0)
	}

	// Create InstallConfig
	installConfig := workflow.InstallConfig{
		WINEPREFIX:        result.WINEPREFIX,
		ProtonPath:        result.ProtonPath,
		GPUType:           result.GPUType,
		GPUCapabilities:   result.GPUCapabilities,
		IsAMDGPU:          result.IsAMDGPU,
		LauncherInstaller: result.LauncherInstaller,
		Workdir:           workdir,
		IsFSR41:           result.UseFSR41,
	}

	// Run installation
	logger.Info("Starting installation phase...")
	installErr := workflow.RunInstaller(installConfig, logger)
	if result.LauncherTempDir != "" {
		_ = os.RemoveAll(result.LauncherTempDir)
	}
	if installErr != nil {
		logger.Error(fmt.Sprintf("Installation failed: %v", installErr))
		os.Exit(1)
	}

	// Run configuration
	configureConfig := workflow.ConfigureConfig{
		WINEPREFIX:      result.WINEPREFIX,
		ProtonPath:      result.ProtonPath,
		GPUType:         result.GPUType,
		GPUCapabilities: result.GPUCapabilities,
		IsAMDGPU:        result.IsAMDGPU,
		Workdir:         workdir,
		IsFSR41:         result.UseFSR41,
	}

	if err := workflow.RunConfiguration(configureConfig, logger); err != nil {
		logger.Error(fmt.Sprintf("Configuration failed: %v", err))
		os.Exit(1)
	}

	logger.Info("Installation complete!")
	fmt.Println()
	fmt.Printf("%sInstallation completed successfully!%s\n", core.ColorBoldGreen, core.ColorReset)
	fmt.Println()
	fmt.Println("You can now launch Bellum using the 'Bellum' using any of these:")
	fmt.Printf("%s", core.Colorize(" - Desktop Shortcut (Recommended)\n", core.Bold))
	fmt.Println(" - Applications Menu -> Games -> Bellum")
	fmt.Println(" - Terminal Command: `Bellum`")
	fmt.Println()
	fmt.Printf("Launch Environment Variable File: %s/launch_vars.env\n", configureConfig.WINEPREFIX)
	fmt.Println()
}

func printInstallerBanner() {
	banner := `=======================================================================================
|                     Linux Wine-Proton Installer for Bellum                          |
======================================================================================`
	fmt.Printf("%s%s%s\n", core.ColorBoldBlue, banner, core.ColorReset)
	fmt.Println()
}
