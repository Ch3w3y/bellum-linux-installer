// Package main provides the entry point for the Bellum installer.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
	"bellum-installer/pkg/packages"
	"bellum-installer/pkg/workflow"
)

func main() {
	if err := core.RequireNonRoot(os.Geteuid()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// The Bellum wrapper runs "update-launcher PREFIX" before each launch.
	if len(os.Args) == 3 && os.Args[1] == "update-launcher" {
		os.Exit(runUpdateLauncher(os.Args[2]))
	}

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
		fmt.Println("  --yes, -y             Accept every default (install to ~/Games/Bellum without asking)")
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

	printInstallerBanner(workdir)
	logger.Record("Bellum Linux Installer " + config.InstallerVersion)

	const steps = 5
	core.Step(1, steps, "Install location")

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
	core.Step(2, steps, "Checking your system")
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

	logger.Info("Display session: " + workflow.DisplaySession())

	core.Step(3, steps, "Review")

	// Print installer summary
	core.PrintInstallerSummary(
		result.ProtonVer,
		config.DefaultVersions.WinetricksVer,
		config.DefaultVersions.VKD3DVer,
		config.DefaultVersions.DXVKVer,
		result.WINEPREFIX,
		result.LauncherInstaller,
		workflow.ProfileSummary(result.Platform),
		result.GPUType,
		workflow.ConfigSummary(result.GPUCapabilities),
		workdir,
	)

	if result.Update {
		fmt.Printf("%sUPDATE:%s the existing install at %s moves to the Proton and settings above.\nThe game, your launcher login and saves are kept.\n", core.ColorBoldYellow, core.ColorReset, result.WINEPREFIX)
	}

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
	core.Step(4, steps, "Downloading the runtime")
	if err := workflow.AcquireRuntime(result, workdir, logger); err != nil {
		if result.LauncherTempDir != "" {
			_ = os.RemoveAll(result.LauncherTempDir)
		}
		fail(logger, logFile, "Downloading or unpacking Proton failed", err, "Check your internet connection and free space in ~/.local/share, then run the installer again. It picks up where it left off.")
	}

	if result.Update {
		core.Step(5, steps, "Updating Bellum")
		runUpdate(result, installConfig, workdir, logFile, logger)
		return
	}

	// Run installation
	core.Step(5, steps, "Installing Bellum")
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
	inSteam := workflow.OfferSteamShortcut(result.Platform, logger)
	printFinish("Bellum is installed", configureConfig.WINEPREFIX, workflow.FinishAdvice(result.Platform, inSteam))
}

// printFinish shows the closing screen with the ways to start the game and,
// on Steam hardware or with a controller, how to play through Steam.
func printFinish(title, prefix string, advice []string) {
	fmt.Println()
	fmt.Printf("  %s✔ %s%s\n\n", core.ColorBoldGreen, title, core.ColorReset)
	fmt.Println("  Start it from:")
	fmt.Printf("    %s▸%s the %sBellum%s shortcut on your desktop\n", core.ColorBoldCyan, core.ColorReset, core.Bold, core.ColorReset)
	fmt.Printf("    %s▸%s your app menu, under Games\n", core.ColorBoldCyan, core.ColorReset)
	if hint := workflow.LocalBinPathHint(); hint != "" {
		fmt.Printf("    %s▸%s the %sBellum%s command, after adding ~/.local/bin to your PATH:\n", core.ColorBoldCyan, core.ColorReset, core.Bold, core.ColorReset)
		fmt.Printf("        %s%s%s\n", core.ColorGrayBold, hint, core.ColorReset)
	} else {
		fmt.Printf("    %s▸%s the %sBellum%s command in a terminal\n", core.ColorBoldCyan, core.ColorReset, core.Bold, core.ColorReset)
	}
	if len(advice) > 0 {
		fmt.Println()
		fmt.Printf("  %s%s%s\n", core.Bold, advice[0], core.ColorReset)
		for _, line := range advice[1:] {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
	fmt.Printf("  %sKeep the Astarte Launcher open while you play. Settings: %s/launch_vars.env%s\n", core.ColorGrayBold, prefix, core.ColorReset)
	fmt.Printf("  %sTo update later, run the same install command again.%s\n\n", core.ColorGrayBold, core.ColorReset)
}

// printInstallerBanner shows the Bellum "B" (rendered from the launcher icon)
// next to the installer and runtime versions.
func printInstallerBanner(workdir string) {
	icon := filepath.Join(workdir, "packages", "launcher_1_256x256x32.png")
	var logo []string
	if packages.VerifySHA256(icon, packages.IconSHA256) == nil {
		logo = core.LogoLines(icon, 26)
	}
	proton := strings.TrimSuffix(strings.TrimPrefix(config.DefaultVersions.ProtonVer, "proton-cachyos-"), "-x86_64")
	winetricks := strings.TrimSuffix(strings.TrimPrefix(config.DefaultVersions.WinetricksVer, "bundled with pinned Proton ("), ")")
	dim := func(s string) string { return core.ColorGrayBold + s + core.ColorReset }
	core.Banner(logo, []string{
		core.ColorBold + "B E L L U M" + core.ColorReset,
		core.ColorBoldCyan + "Linux Installer" + core.ColorReset + " " + dim(config.InstallerVersion),
		"",
		dim("Proton-CachyOS ") + proton,
		dim("umu-launcher   ") + config.DefaultVersions.UMUVersion,
		dim("winetricks     ") + winetricks,
		"",
		dim("Community project, not affiliated with Astarte Industries"),
		dim("github.com/Ch3w3y/bellum-linux-installer"),
	})
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
	fmt.Printf("\n  %sWhat to do:%s %s\n", core.ColorBoldYellow, core.ColorReset, fix)
	fmt.Printf("  %sFull log: %s%s\n\n", core.ColorGrayBold, logFile, core.ColorReset)
	os.Exit(1)
}

// runUpdate refreshes a finished install onto the current pins without
// touching the prefix contents, then exits.
func runUpdate(result *workflow.PrecheckResult, installConfig workflow.InstallConfig, workdir, logFile string, logger *core.Logger) {
	if result.LauncherTempDir != "" {
		defer os.RemoveAll(result.LauncherTempDir)
	}
	if err := workflow.RunUpdate(installConfig, logger); err != nil {
		fail(logger, logFile, "Updating the Bellum launcher failed", err, "Your install is unchanged apart from Proton being downloaded. Run the installer again to retry.")
	}
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
		fail(logger, logFile, "Writing Bellum's updated launch settings failed", err, "Run the installer again to retry. The game and your login are untouched.")
	}
	logger.Info("Update complete!")
	inSteam := workflow.OfferSteamShortcut(result.Platform, logger)
	printFinish("Bellum is up to date", result.WINEPREFIX, workflow.FinishAdvice(result.Platform, inSteam))
}

// runUpdateLauncher installs the latest Astarte Launcher into prefix. Its
// output goes to the wrapper's launcher.log.
func runUpdateLauncher(prefix string) int {
	logger, err := core.NewLogger("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer logger.Close()
	if err := workflow.UpdateLauncherInPrefix(prefix, logger); err != nil {
		logger.Error(err.Error())
		return 1
	}
	return 0
}
