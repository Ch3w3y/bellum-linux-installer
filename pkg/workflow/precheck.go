package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
	"bellum-installer/pkg/gui"
	"bellum-installer/pkg/packages"
)

// PrecheckResult holds the result of precheck validation
type PrecheckResult struct {
	WINEPREFIX        string
	WINEPREFIXSource  string
	UseExisting       bool
	ForceWineVersion  bool
	LauncherInstaller string
	LauncherTempDir   string
	GPUType           string
	IsAMDGPU          bool
	GPUCapabilities   core.GPUCapabilities
	UseFSR41          bool
	ProtonVer         string
	ProtonPath        string
}

// ValidateWINEPREFIX validates the WINEPREFIX path
// Uses GUI directory picker if no argument is provided
// The WINEPREFIX is always stored at <selectedPath>/Bellum
func ValidateWINEPREFIX(wineprefixArg string, logger *core.Logger) (string, string, bool, error) {
	WINEPREFIX := ""
	WINEPREFIXSource := ""
	UseExisting := false

	if wineprefixArg != "" {
		// If user provides a path, check if it ends with "Bellum"
		// If not, append "Bellum" to create the WINEPREFIX path
		WINEPREFIX = strings.TrimSuffix(wineprefixArg, "/")
		if !strings.HasSuffix(WINEPREFIX, "Bellum") {
			WINEPREFIX = filepath.Join(WINEPREFIX, "Bellum")
		}
		WINEPREFIXSource = "argument"
		logger.Info(fmt.Sprintf("WINEPREFIX: %s%s%s", core.ColorBoldYellow, WINEPREFIX, core.ColorReset))
	} else if envPrefix := os.Getenv("WINEPREFIX"); envPrefix != "" {
		WINEPREFIX = envPrefix
		WINEPREFIXSource = "environment variable"
		logger.Info(fmt.Sprintf("WINEPREFIX is already set to: %s%s%s", core.ColorBoldYellow, WINEPREFIX, core.ColorReset))
		if core.AskBool("Do you want to use this path? (Y/n): ") {
			UseExisting = true
		} else {
			logger.Info("Enter the desired WINEPREFIX path (e.g. /path/to/wineprefix):")
			reader := core.NewReader()
			input, err := reader.ReadString('\n')
			if err != nil {
				return "", "", false, fmt.Errorf("failed to read WINEPREFIX: %w", err)
			}
			WINEPREFIX = strings.TrimSpace(input)
			WINEPREFIXSource = "user input"
		}
	} else {
		// Use GUI directory picker
		logger.Info("Select the directory where you want to install Bellum...")
		logger.Info("This will create a new WINEPREFIX named 'Bellum' in the selected location.")
		fmt.Println()

		result, err := gui.PickDirectory("")
		if err != nil {
			return "", "", false, fmt.Errorf("failed to pick directory: %w", err)
		}

		if !result.Success {
			return "", "", false, fmt.Errorf("directory selection cancelled or failed: %v", result.Error)
		}

		// The GUI picker returns the parent path, we need to append "Bellum"
		selectedPath := strings.TrimSuffix(result.Path, "/")
		WINEPREFIX = wineprefixForSelectedDirectory(selectedPath)
		WINEPREFIXSource = "GUI picker"

		logger.Info(fmt.Sprintf("WINEPREFIX: %s%s%s", core.ColorBoldYellow, WINEPREFIX, core.ColorReset))
	}

	// Normalize path
	WINEPREFIX = strings.TrimSuffix(WINEPREFIX, "/")

	// Validate absolute path
	if !strings.HasPrefix(WINEPREFIX, "/") {
		return "", "", false, fmt.Errorf("WINEPREFIX must be an absolute path (starting with /): %s", WINEPREFIX)
	}

	// Check parent directory exists and is writable
	WINEPREFIXParent := WINEPREFIX
	for !isDir(WINEPREFIXParent) && WINEPREFIXParent != "/" {
		WINEPREFIXParent = filepath.Dir(WINEPREFIXParent)
	}

	if !isDir(WINEPREFIXParent) {
		return "", "", false, fmt.Errorf("WINEPREFIX path is not on a valid mounted filesystem: %s", WINEPREFIX)
	}

	// Check if WINEPREFIX exists (Bellum directory)
	exists := isDir(WINEPREFIX)
	wineprefixExists := false

	if exists {
		entries, err := os.ReadDir(WINEPREFIX)
		if err == nil && len(entries) != 0 {
			wineprefixExists = true
		}
	}

	// Directory is empty, remove it and re-download

	if wineprefixExists {
		// Check if it's a valid Bellum installation by looking for typical Bellum files
		// If it exists but looks like an empty directory or invalid prefix, allow reinstallation
		logger.Info(fmt.Sprintf("WINEPREFIX directory already exists at %s", WINEPREFIX))
		logger.Warn("If you want to reinstall, please uninstall the existing installation first.")
		return "", "", false, fmt.Errorf("WINEPREFIX directory  '%s' already exists", WINEPREFIX)
	}

	if !isWritable(WINEPREFIXParent) {
		return "", "", false, fmt.Errorf("WINEPREFIX parent directory is not writable: %s", WINEPREFIXParent)
	}

	logger.Info("[OK] WINEPREFIX path is valid and writable")

	// Check if SSD - use the parent directory (where Bellum is located)
	if isSSD(WINEPREFIXParent, logger) {
		logger.Info("[OK] WINEPREFIX device is an SSD/NVME (optimal performance)")
	} else {
		logger.Warn("WINEPREFIX device is NOT an SSD/NVME (may have performance issues)")
		if !core.AskBool("Astarte Developers strongly recommend using NVMe or SSD for the game. Are you sure you want to proceed? (Y/n): ") {
			return "", "", false, fmt.Errorf("installation cancelled by user")
		}
	}

	return WINEPREFIX, WINEPREFIXSource, UseExisting, nil
}

// ValidateWINEPREFIXWithGUI prompts user to select a directory using GUI picker and validates it
// This function handles the complete workflow of:
// 1. Opening GUI directory picker
// 2. Validating the selected directory
// 3. Creating the Bellum directory inside the selected path
// Returns the WINEPREFIX path (which is <selectedPath>/Bellum)
func ValidateWINEPREFIXWithGUI(logger *core.Logger) (string, error) {
	// Open GUI directory picker
	fmt.Println()
	logger.Info("Select the directory where you want to install Bellum...")
	logger.Info("This will create a new WINEPREFIX named 'Bellum' in the selected location.")
	time.Sleep(2 * time.Second)

	result, err := gui.PickDirectory("")
	if err != nil {
		return "", fmt.Errorf("failed to pick directory: %w", err)
	}

	if !result.Success {
		return "", fmt.Errorf("directory selection cancelled or failed: %v", result.Error)
	}

	selectedPath := result.Path
	logger.Info(fmt.Sprintf("Selected directory: %s", core.Colorize(selectedPath, core.ColorBoldYellow)))
	fmt.Println()

	// The WINEPREFIX will be created at selectedPath/Bellum
	wineprefixPath := wineprefixForSelectedDirectory(selectedPath)

	// Validate the selected directory
	valid, errMsg := gui.ValidateDirectory(wineprefixPath, logger)
	if !valid {
		return "", fmt.Errorf("directory validation failed: %s", errMsg)
	}

	logger.Info("[OK] Directory validation passed")
	fmt.Println()

	return wineprefixPath, nil
}

func wineprefixForSelectedDirectory(selectedPath string) string {
	return filepath.Join(selectedPath, "Bellum")
}

// CheckRequiredWineBinaries checks if all required Wine binaries are present
func CheckRequiredWineBinaries(logger *core.Logger) error {
	return checkRequiredWineBinaries(DefaultBoundaries.Files, logger)
}

func checkRequiredWineBinaries(files FileStore, logger *core.Logger) error {
	return checkRequiredWineBinariesWith(files, DefaultBoundaries.Commands, logger)
}

func checkRequiredWineBinariesWith(files FileStore, commands CommandRunner, logger *core.Logger) error {
	requiredBinaries := []string{
		config.DefaultVersions.Binaries.Wine,
		config.DefaultVersions.Binaries.Wineboot,
		config.DefaultVersions.Binaries.Msidb,
		config.DefaultVersions.Binaries.Winecfg,
		config.DefaultVersions.Binaries.Wineserver,
	}

	var missing []string
	for _, binary := range requiredBinaries {
		if DiscoverExecutable(binary, commands) == "" {
			missing = append(missing, binary)
		}
	}

	if len(missing) > 0 {
		host := DetectHost(files, commands)
		guidance := MissingDependencyGuidance(host, missing)
		logger.Error("Required Wine tools not found in PATH: " + strings.Join(missing, ", "))
		logger.Error(guidance)
		return fmt.Errorf("missing Wine binaries: %s", guidance)
	}

	logger.Info("[OK] All required Wine binaries found")
	return nil
}

// CheckWineVersion verifies Wine version matches requirements
func CheckWineVersion(logger *core.Logger, force bool) error {
	installedWine := getWineVersionWith(DefaultBoundaries.Commands, logger)
	requiredWine := strings.TrimPrefix(config.DefaultVersions.WineVer, "wine-")

	if installedWine == "" {
		return fmt.Errorf("Wine binary not found in PATH")
	}

	if installedWine != requiredWine {
		if !force {
			logger.Error(fmt.Sprintf("Wine version mismatch. Installed: wine-%s, Required: %s", installedWine, requiredWine))
			return fmt.Errorf("wine version mismatch: installed %s, required %s", installedWine, requiredWine)
		}
		logger.Warn(fmt.Sprintf("Wine version mismatch. Installed: wine-%s, Required: %s", installedWine, requiredWine))
		logger.Warn("Proceeding with wine-1.0 due to --force-wine-version flag (not recommended)")
	} else {
		logger.Info(fmt.Sprintf("[OK] Wine %s stable found", requiredWine))
	}

	return nil
}

// CheckUMURun verifies umu-run is available
func CheckUMURun(logger *core.Logger) error {
	return checkUMURun(DefaultBoundaries.Commands, logger)
}

func checkUMURun(commands CommandRunner, logger *core.Logger) error {
	if DiscoverExecutable("umu-run", commands) == "" {
		logger.Error("umu-run binary not found in PATH.\nGrab latest umu-launcher-1.3.0 for your distro: https://github.com/Open-Wine-Components/umu-launcher/releases/tag/1.3.0")
		return fmt.Errorf("umu-run not found")
	}

	logger.Info("[OK] umu-run binary found: " + DiscoverExecutable("umu-run", commands))
	return nil
}

// CheckLauncherInstaller checks if launcher installer is available
func CheckLauncherInstaller(launcherInstallerPath string, logger *core.Logger) error {
	return checkLauncherInstaller(launcherInstallerPath, logger, DefaultBoundaries.Files, DefaultBoundaries.Commands)
}

func checkLauncherInstaller(launcherInstallerPath string, logger *core.Logger, files FileStore, commands CommandRunner) error {
	if config.DefaultVersions.LauncherSigner == "" {
		return fmt.Errorf("AstarteLauncher Authenticode signer pin is required")
	}
	if DiscoverExecutable("osslsigncode", commands) == "" {
		return fmt.Errorf("osslsigncode is required to verify the launcher installer")
	}
	if launcherInstallerPath != "" {
		if _, err := files.Stat(launcherInstallerPath); os.IsNotExist(err) {
			logger.Error(fmt.Sprintf("Launcher installer not found at: %s", launcherInstallerPath))
			return fmt.Errorf("launcher installer not found: %s", launcherInstallerPath)
		}
		if err := packages.VerifyLauncherInstaller(launcherInstallerPath); err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("[OK] Launcher installer found: %s", launcherInstallerPath))
		return nil
	}

	if DiscoverExecutable("wget", commands) == "" {
		logger.Error("Launcher installer path not provided and wget is not available.")
		return fmt.Errorf("launcher installer not provided and wget not available")
	}

	logger.Info("[OK] wget found for launcher installer download")
	return nil
}

// CheckWinetricks checks if winetricks is available
func CheckWinetricks(workdir string, logger *core.Logger) error {
	if DetectHost(DefaultBoundaries.Files, DefaultBoundaries.Commands).Immutable {
		if DiscoverExecutable("winetricks", DefaultBoundaries.Commands) == "" {
			message := MissingDependencyGuidance(DetectHost(DefaultBoundaries.Files, DefaultBoundaries.Commands), []string{"winetricks"})
			logger.Error(message)
			return fmt.Errorf("winetricks unavailable on immutable host: %s", message)
		}
	}
	logger.Info("Installing pinned vendored winetricks without privilege...")

	winetricksArchive := filepath.Join(workdir, "packages", "winetricks-"+config.DefaultVersions.WinetricksVer+".tar.gz")
	if _, err := os.Stat(winetricksArchive); os.IsNotExist(err) {
		logger.Error(fmt.Sprintf("winetricks binary not found in PATH and %s not found", winetricksArchive))
		return fmt.Errorf("winetricks not found")
	}
	privateDir, err := os.MkdirTemp("", "bellum-winetricks-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(privateDir)
	if err := os.Chmod(privateDir, 0700); err != nil {
		return err
	}
	verifiedArchive := filepath.Join(privateDir, filepath.Base(winetricksArchive))
	if err := packages.CopyVerifiedSHA256(winetricksArchive, verifiedArchive, packages.WinetricksSHA256); err != nil {
		return err
	}

	logger.Info("Extracting verified winetricks copy into a private temporary directory...")
	tmpDir, err := packages.ExtractPackage(verifiedArchive, "winetricks")
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to extract %s: %v", winetricksArchive, err))
		return err
	}

	if !isDir(tmpDir) {
		logger.Error(fmt.Sprintf("Expected directory %s not found after extraction", tmpDir))
		return fmt.Errorf("winetricks extraction failed")
	}

	winetricks, err := os.ReadFile(filepath.Join(tmpDir, "src", "winetricks"))
	if err != nil {
		return fmt.Errorf("read vendored winetricks: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(binDir, "winetricks"), winetricks, 0755); err != nil {
		return fmt.Errorf("install unprivileged winetricks: %w", err)
	}
	logger.Info("[OK] Vendored winetricks installed in ~/.local/bin")

	logger.Info("Cleaning up extracted winetricks directory...")
	packages.CleanupTempDir(winetricksArchive)

	return nil
}

// CheckProton ensures Proton is available
func CheckProton(packageRoot string, gpuType string, isFSR41 bool, logger *core.Logger) (string, string, error) {
	if config.DefaultVersions.ProtonSHA256 == "" {
		return "", "", fmt.Errorf("approved Proton SHA-256 pin is required")
	}
	isAMD := strings.Contains(strings.ToLower(gpuType), "amd") || strings.Contains(strings.ToLower(gpuType), "radeon")

	if DiscoverExecutable("wget", DefaultBoundaries.Commands) == "" {
		logger.Error("Proton is missing and wget is not available to download it.")
		return "", "", fmt.Errorf("proton missing and wget not available")
	}
	logger.Info("[OK] wget found for Proton download")

	// Use proton-cachyos for all GPUs (AMD and NVIDIA)
	protonVer := config.DefaultVersions.ProtonVer
	_ = packages.GetProtonURL(protonVer, config.DefaultVersions.ProtonBaseURL)

	// Get the actual proton install path
	protonDir := packages.GetProtonInstallPath(protonVer)

	if err := packages.EnsureProtonWithLog(protonDir, protonVer, isAMD, isFSR41, filepath.Join(filepath.Dir(packageRoot), "logs", "installer.log"), logger); err != nil {
		return "", "", err
	}

	return protonVer, protonDir, nil
}

// DetectGPU detects the GPU type
func DetectGPU(logger *core.Logger) (string, error) {
	gpuType, err := core.DetectGPU()
	if err != nil {
		logger.Error("Failed to detect GPU type")
		return "", fmt.Errorf("failed to detect GPU type: %w", err)
	}

	logger.Info(fmt.Sprintf("GPU Vendor: %s", gpuType))
	return gpuType, nil
}

// RunPrechecks runs all precheck validations
func RunPrechecks(wineprefixArg string, launcherInstallerPath string, forceWineVersion bool, fsr41 bool, logger *core.Logger) (*PrecheckResult, error) {
	logger.Info("Starting precheck phase...")
	fmt.Println()

	// Detect GPU type
	gpuCaps, err := core.DetectGPUCapabilities()
	if err != nil {
		return nil, err
	}
	gpuType := string(gpuCaps.Vendor)
	if gpuCaps.Vendor == core.GPUUnknown {
		gpuType = "Unknown"
	}
	isAMD := gpuCaps.Vendor == core.GPUAMD
	// RDNA4 needs the Proton driver component for the game's native FSR4.
	// The retired fsr41 CLI option is ignored; RDNA3 remains disabled.
	useFSR41 := gpuCaps.Vendor == core.GPUAMD && gpuCaps.Generation == "RDNA4" && !gpuCaps.Ambiguous
	logger.Info(fmt.Sprintf("GPU Vendor: %s (generation: %s, ambiguous: %t)", gpuType, gpuCaps.Generation, gpuCaps.Ambiguous))
	_ = fsr41

	// Validate WINEPREFIX
	wineprefix, _, _, err := ValidateWINEPREFIX(wineprefixArg, logger)
	if err != nil {
		return nil, err
	}

	// Check Wine binaries
	if err := checkRequiredWineBinaries(DefaultBoundaries.Files, logger); err != nil {
		return nil, err
	}

	// Check Wine version
	if err := CheckWineVersion(logger, forceWineVersion); err != nil {
		return nil, err
	}

	// Check xdotool (optional - commented out per bash version)
	// if err := CheckXdotool(logger); err != nil {
	// 	return nil, err
	// }

	// Check umu-run
	if err := CheckUMURun(logger); err != nil {
		return nil, err
	}

	// Check precheck dependencies and the supplied artifact. The latter check is
	// followed by a private copy that is independently hashed and verified.
	if err := CheckLauncherInstaller(launcherInstallerPath, logger); err != nil {
		return nil, err
	}
	// Stage and verify a user-supplied launcher before the later install phase.
	var launcherTempDir string
	stagedLauncher := launcherInstallerPath
	if launcherInstallerPath != "" {
		var stageErr error
		stagedLauncher, launcherTempDir, stageErr = packages.StageLauncherInstaller(launcherInstallerPath)
		if stageErr != nil {
			return nil, stageErr
		}
	}
	keepLauncher := false
	defer func() {
		if !keepLauncher && launcherTempDir != "" {
			_ = os.RemoveAll(launcherTempDir)
		}
	}()
	// Check winetricks
	if err := CheckWinetricks(".", logger); err != nil {
		return nil, err
	}

	packageRoot, err := filepath.Abs("./packages")
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of workroot: %w", err)
	}

	// Check Proton
	protonVer, protonPath, err := CheckProton(packageRoot, gpuType, useFSR41, logger)
	if err != nil {
		return nil, err
	}
	runtimePath := eacRuntimePath()
	if info, err := os.Stat(runtimePath); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("Proton EasyAntiCheat Runtime is required at %q (set PROTON_EAC_RUNTIME to its installed directory)", runtimePath)
	}
	if err := packages.VerifyEACRuntime(runtimePath, config.DefaultVersions.EACRuntimeSHA256Allowlist); err != nil {
		return nil, err
	}

	logger.Info("[OK] All prechecks passed!")
	fmt.Println()
	keepLauncher = true

	return &PrecheckResult{
		WINEPREFIX:        wineprefix,
		GPUType:           gpuType,
		IsAMDGPU:          isAMD,
		GPUCapabilities:   gpuCaps,
		UseFSR41:          useFSR41,
		ProtonVer:         protonVer,
		ProtonPath:        protonPath,
		ForceWineVersion:  forceWineVersion,
		LauncherInstaller: stagedLauncher,
		LauncherTempDir:   launcherTempDir,
	}, nil
}

// Helper functions

func isDir(path string) bool {
	return isDirWith(path, DefaultBoundaries.Files)
}

func isDirWith(path string, files FileStore) bool {
	info, err := files.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isWritable(path string) bool {
	file, err := os.OpenFile(filepath.Join(path, ".write_test"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false
	}
	defer os.Remove(filepath.Join(path, ".write_test"))
	file.Close()
	return true
}

func isSSD(path string, logger *core.Logger) bool {
	return isSSDWith(path, logger, DefaultBoundaries.Commands)
}

func isSSDWith(path string, logger *core.Logger, commands CommandRunner) bool {
	// Try lsblk first
	if output, err := commands.Output([]string{"lsblk", "-no", "rota", filepath.Dir(path)}); err == nil {
		rotational := strings.TrimSpace(output)
		return rotational == "0"
	}

	// Fallback to checking device name
	device, err := commands.Output([]string{"df", "-P", path})
	if err != nil {
		return false
	}

	// Parse device name from df output
	lines := strings.Split(device, "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 1 {
			deviceName := filepath.Base(fields[0])
			return strings.HasPrefix(deviceName, "nvme") || strings.HasPrefix(deviceName, "sd") || strings.HasPrefix(deviceName, "vd")
		}
	}

	return false
}

func getWineVersion(logger *core.Logger) string {
	return getWineVersionWith(DefaultBoundaries.Commands, logger)
}

func getWineVersionWith(commands CommandRunner, logger *core.Logger) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	defaultPfx := filepath.Join(homeDir, ".wine")
	os.Setenv("WINEPREFIX", defaultPfx)
	output, err := commands.Output([]string{"wine", "--version"})
	if err != nil {
		return ""
	}

	// Extract version number using regex
	versionRegex := regexp.MustCompile(`wine-([0-9.]+)`)
	match := versionRegex.FindStringSubmatch(output)
	if len(match) >= 2 {
		return match[1]
	}

	return ""
}

// Scanner for user input
type Scanner struct {
	reader *core.Scanner
}

func NewScanner() *Scanner {
	return &Scanner{reader: core.NewReader()}
}

func (s *Scanner) ReadString(delim byte) (string, error) {
	return s.reader.ReadString(delim)
}
