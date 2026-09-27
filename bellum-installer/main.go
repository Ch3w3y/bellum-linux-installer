// Package main provides the entry point for the Bellum installer.
package main

import (
	"errors"
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
	// Accepted for compatibility with older instructions; system Wine is no
	// longer used, so there is no version to force.
	forceWineVersion := flag.Bool("force-wine-version", false, "Deprecated; has no effect")
	wineprefix := flag.String("wineprefix", "", "Path to WINEPREFIX directory (optional if WINEPREFIX env var is set)")
	launcherInstaller := flag.String("launcher-installer", "", "Path to launcher installer executable")
	assumeYes := flag.Bool("yes", false, "Accept every default without asking")
	flag.BoolVar(assumeYes, "y", false, "Shorthand for --yes")
	help := flag.Bool("help", false, "Show help message")

	flag.Parse()
	core.AssumeYes = *assumeYes

	if *help {
		fmt.Println("Bellum Linux Installer")
		fmt.Println()
		fmt.Println("Usage: bellum-installer [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --wineprefix PATH     Install location (default: ask, suggesting ~/Games/Bellum)")
		fmt.Println("  --launcher-installer PATH  Use a local Astarte Launcher installer (still verified)")
		fmt.Println("  --yes, -y             Accept every default: ~/Games/Bellum, Stable preset, no extras")
		fmt.Println("  --help                Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  bellum-installer")
		fmt.Println("  bellum-installer --wineprefix ~/Games --yes")
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

	// Determine WINEPREFIX. Flag, environment and GUI all resolve through
	// workflow.ResolvePrefixPath, which appends "Bellum" when needed.
	var selectedWINEPREFIX string
	if *wineprefix != "" {
		logger.Info(fmt.Sprintf("Install location from flag: %s", core.Colorize(*wineprefix, core.ColorBoldYellow)))
		selectedWINEPREFIX, err = workflow.ResolvePrefixPath(*wineprefix)
	} else if envPrefix := os.Getenv("WINEPREFIX"); envPrefix != "" {
		logger.Info(fmt.Sprintf("Install location from environment: %s", core.Colorize(envPrefix, core.ColorBoldYellow)))
		selectedWINEPREFIX, err = workflow.ResolvePrefixPath(envPrefix)
	} else {
		selectedWINEPREFIX, err = workflow.PromptInstallLocation(logger)
	}
	if err != nil {
		fail(logger, logFile, "Couldn't choose an install location", err, "Run the installer again and pick a folder you own, or pass --wineprefix ~/Games.")
	}

	if *forceWineVersion {
		logger.Warn("--force-wine-version is deprecated and has no effect: Bellum no longer uses system Wine.")
	}

	// Read-only prechecks: nothing is downloaded or written to $HOME yet.
	launcherPath := *launcherInstaller
	if launcherPath != "" {
		if abs, absErr := filepath.Abs(launcherPath); absErr == nil {
			launcherPath = abs
		}
	}
	result, err := workflow.RunPrechecks(workflow.PrecheckOptions{
		Wineprefix:        selectedWINEPREFIX,
		LauncherInstaller: launcherPath,
		Workdir:           workdir,
	}, logger)
	if err != nil {
		fail(logger, logFile, "Bellum can't be installed yet", err, "Fix the problems listed above, then run the installer again. Nothing on this system was changed.")
	}

	// Guided choices; pressing Enter keeps the Stable preset and no extras.
	options := workflow.ChooseInstallOptions(result.GPUCapabilities, logger)

	// Print installer summary
	core.PrintInstallerSummary(
		result.ProtonVer,
		config.DefaultVersions.WinetricksVer,
		config.DefaultVersions.VKD3DVer,
		config.DefaultVersions.DXVKVer,
		result.WINEPREFIX,
		result.LauncherInstaller,
		result.GPUType,
		options.Summary(),
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
		ReplaceIncomplete: result.ReplaceIncomplete,
	}

	// Acquire the pinned Proton only now that the user has confirmed.
	logger.Info("Preparing Proton...")
	if err := workflow.AcquireRuntime(result, workdir, logger); err != nil {
		if result.LauncherTempDir != "" {
			_ = os.RemoveAll(result.LauncherTempDir)
		}
		fail(logger, logFile, "Downloading or unpacking Proton failed", err, "Check your internet connection and free space in ~/.local/share, then run the installer again. It picks up where it left off.")
	}

	// Run installation
	logger.Info("Starting installation phase...")
	installErr := workflow.RunInstaller(installConfig, logger)
	if result.LauncherTempDir != "" {
		_ = os.RemoveAll(result.LauncherTempDir)
	}
	if installErr != nil {
		fail(logger, logFile, "Setting up the Bellum prefix failed", installErr, "The unfinished install was cleaned up. Run the installer again to retry; if it fails at the same step, include the log below when asking for help.")
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
		Options:         options,
	}

	if err := workflow.RunConfiguration(configureConfig, logger); err != nil {
		if rmErr := workflow.DiscardIncompleteInstall(result.WINEPREFIX, logger); rmErr != nil {
			logger.Warn(fmt.Sprintf("Could not remove the unfinished install; re-running the installer will offer to start over: %v", rmErr))
		}
		fail(logger, logFile, "Writing Bellum's launch settings failed", err, "Run the installer again to retry.")
	}
	if err := workflow.MarkInstallComplete(result.WINEPREFIX); err != nil {
		fail(logger, logFile, "Couldn't mark the install as finished", err, "Check that you can write to "+result.WINEPREFIX+", then run the installer again.")
	}

	logger.Info("Installation complete!")
	fmt.Println()
	fmt.Printf("%sInstallation completed successfully!%s\n", core.ColorBoldGreen, core.ColorReset)
	fmt.Println()
	fmt.Println("You can now launch Bellum in any of these ways:")
	fmt.Printf("%s", core.Colorize(" - Desktop Shortcut (Recommended)\n", core.Bold))
	fmt.Println(" - Applications Menu -> Games -> Bellum")
	if hint := workflow.LocalBinPathHint(); hint != "" {
		fmt.Println(" - Terminal Command: `Bellum` (after adding ~/.local/bin to your PATH:")
		fmt.Printf("     %s )\n", hint)
	} else {
		fmt.Println(" - Terminal Command: `Bellum`")
	}
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

// fail reports a fatal error in plain language: what went wrong, the detail,
// what to do next and where the full log is. It exits the process.
func fail(logger *core.Logger, logFile, what string, err error, fix string) {
	fmt.Println()
	logger.Error(what + ".")
	var precheckErr *workflow.PrecheckError
	if !errors.As(err, &precheckErr) {
		logger.Error(fmt.Sprintf("Details: %v", err))
	}
	fmt.Printf("%sWhat to do:%s %s\n", core.ColorBoldYellow, core.ColorReset, fix)
	fmt.Printf("Full log: %s\n", logFile)
	os.Exit(1)
}
