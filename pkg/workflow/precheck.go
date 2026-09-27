package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
	"bellum-installer/pkg/gui"
	"bellum-installer/pkg/packages"
)

// PrecheckResult holds the result of precheck validation
type PrecheckResult struct {
	WINEPREFIX        string
	ReplaceIncomplete bool
	LauncherInstaller string
	LauncherTempDir   string
	GPUType           string
	IsAMDGPU          bool
	GPUCapabilities   core.GPUCapabilities
	UseFSR41          bool
	ProtonVer         string
	ProtonPath        string
}

// ResolvePrefixPath turns a user-supplied location into the Bellum prefix
// path. It is the only place that appends "Bellum", so the flag, the
// environment variable and the GUI picker all behave the same way: the
// result is absolute, cleaned and always ends in a "Bellum" directory.
func ResolvePrefixPath(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("no install location given")
	}
	abs, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("resolve install location %q: %w", input, err)
	}
	if filepath.Base(abs) != "Bellum" {
		abs = filepath.Join(abs, "Bellum")
	}
	return abs, nil
}

// prefixState classifies what already exists at a resolved prefix path.
type prefixState int

const (
	prefixAbsent     prefixState = iota // nothing there yet
	prefixEmpty                         // an empty directory we can use
	prefixIncomplete                    // our manifest plus the install-incomplete marker
	prefixInstalled                     // our manifest, install finished
	prefixForeign                       // something else; never touch it
)

func inspectPrefix(prefix string, files FileStore) (prefixState, error) {
	info, err := os.Lstat(prefix)
	if os.IsNotExist(err) {
		return prefixAbsent, nil
	}
	if err != nil {
		return prefixForeign, err
	}
	if !info.IsDir() {
		return prefixForeign, nil
	}
	entries, err := files.ReadDir(prefix)
	if err != nil {
		return prefixForeign, err
	}
	if len(entries) == 0 {
		return prefixEmpty, nil
	}
	if _, err := readManifest(prefix, files); err != nil {
		return prefixForeign, nil
	}
	if _, err := files.Stat(filepath.Join(prefix, incompleteMarkerName)); err == nil {
		return prefixIncomplete, nil
	}
	return prefixInstalled, nil
}

// ValidateWINEPREFIX checks a resolved prefix path (see ResolvePrefixPath)
// without creating or changing anything. It reports whether the caller should
// replace an unfinished earlier install once the user has confirmed.
func ValidateWINEPREFIX(wineprefix string, logger *core.Logger) (string, bool, error) {
	return validateWINEPREFIXWith(wineprefix, logger, DefaultBoundaries.Files, core.AskBool)
}

func validateWINEPREFIXWith(wineprefix string, logger *core.Logger, files FileStore, ask func(string) bool) (string, bool, error) {
	if wineprefix == "" || !filepath.IsAbs(wineprefix) || filepath.Base(wineprefix) != "Bellum" {
		return "", false, fmt.Errorf("WINEPREFIX must be an absolute path ending in Bellum: %q", wineprefix)
	}
	wineprefix = filepath.Clean(wineprefix)
	logger.Info(fmt.Sprintf("WINEPREFIX: %s", core.Colorize(wineprefix, core.ColorBoldYellow)))

	// Find the nearest existing ancestor; the prefix is created beneath it later.
	parent := wineprefix
	for !isDirWith(parent, files) && parent != "/" {
		parent = filepath.Dir(parent)
	}
	if !isDirWith(parent, files) {
		return "", false, fmt.Errorf("WINEPREFIX path is not on a valid mounted filesystem: %s", wineprefix)
	}

	replaceIncomplete := false
	state, err := inspectPrefix(wineprefix, files)
	if err != nil {
		return "", false, fmt.Errorf("inspect %s: %w", wineprefix, err)
	}
	switch state {
	case prefixIncomplete:
		logger.Warn(fmt.Sprintf("A previous Bellum install at %s didn't finish.", wineprefix))
		if !ask("Start over? Everything in that folder will be replaced. (Y/n): ") {
			return "", false, fmt.Errorf("installation cancelled: the unfinished install at %s was kept", wineprefix)
		}
		replaceIncomplete = true
	case prefixInstalled:
		return "", false, fmt.Errorf("Bellum is already installed at %s. To reinstall, run the uninstaller first: ./uninstaller --wineprefix %s", wineprefix, wineprefix)
	case prefixForeign:
		return "", false, fmt.Errorf("%s already exists and is not an empty folder or a Bellum install. Choose another location or move that folder away", wineprefix)
	}

	if !isWritable(parent) {
		return "", false, fmt.Errorf("WINEPREFIX parent directory is not writable: %s", parent)
	}
	logger.Info("[OK] WINEPREFIX path is valid and writable")

	if isSSD(parent, logger) {
		logger.Info("[OK] WINEPREFIX device is an SSD/NVME (optimal performance)")
	} else {
		logger.Warn("WINEPREFIX device is NOT an SSD/NVME (may have performance issues)")
		if !ask("Astarte Developers strongly recommend using NVMe or SSD for the game. Are you sure you want to proceed? (Y/n): ") {
			return "", false, fmt.Errorf("installation cancelled by user")
		}
	}

	return wineprefix, replaceIncomplete, nil
}

// PickWINEPREFIXWithGUI opens the folder picker and returns the resolved
// prefix path (<picked>/Bellum). It never creates directories; validation and
// creation happen later, after the user confirms the install.
func PickWINEPREFIXWithGUI(logger *core.Logger) (string, error) {
	fmt.Println()
	logger.Info("Select the folder to install Bellum into...")
	logger.Info("A 'Bellum' folder will be created inside the folder you pick.")
	time.Sleep(2 * time.Second)

	result, err := gui.PickDirectory("")
	if err != nil {
		return "", fmt.Errorf("failed to pick directory: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("directory selection cancelled or failed: %v", result.Error)
	}
	logger.Info(fmt.Sprintf("Selected directory: %s", core.Colorize(result.Path, core.ColorBoldYellow)))
	fmt.Println()
	return ResolvePrefixPath(result.Path)
}

// Free-space floors. The prefix gets .NET, the VC++ runtime and WebView2
// before the launcher downloads the game itself; Proton unpacks to roughly
// 1.5 GB; the compressed archive is staged in the temporary directory.
const (
	minPrefixFreeBytes = 10 << 30
	minProtonFreeBytes = 3 << 30
	minTempFreeBytes   = 1 << 30
)

// PrecheckOptions are the inputs RunPrechecks needs.
type PrecheckOptions struct {
	// Wineprefix is already resolved by ResolvePrefixPath.
	Wineprefix        string
	LauncherInstaller string
	// Workdir is the directory holding the installer binary and packages/.
	Workdir string
}

// precheckHost holds every host effect RunPrechecks reaches, so tests can run
// the full precheck phase against fakes.
type precheckHost struct {
	Commands      CommandRunner
	Files         FileStore
	DetectGPU     func() (core.GPUCapabilities, error)
	Ask           func(string) bool
	FreeBytes     func(string) (uint64, error)
	VerifyEAC     func(string, []string) (string, bool, error)
	StageLauncher func(string) (string, string, packages.LauncherCheck, error)
	ProtonDir     func(string) string
	FindEAC       func() (EACRuntime, error)
}

var defaultPrecheckHost = precheckHost{
	Commands:      DefaultBoundaries.Commands,
	Files:         DefaultBoundaries.Files,
	DetectGPU:     core.DetectGPUCapabilities,
	Ask:           core.AskBool,
	FreeBytes:     freeBytes,
	VerifyEAC:     packages.VerifyEACRuntime,
	StageLauncher: packages.StageLauncherInstaller,
	ProtonDir:     packages.GetProtonInstallPath,
	FindEAC: func() (EACRuntime, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return EACRuntime{}, err
		}
		return findEACRuntime(home, os.Getenv("PROTON_EAC_RUNTIME"), DefaultBoundaries.Files)
	},
}

// requiredTools lists the host commands the install needs. Every Wine
// operation runs through umu-run and the pinned Proton, so no system Wine or
// winetricks is required.
func requiredTools() []string {
	return []string{"umu-run", "osslsigncode", "wget"}
}

// RunPrechecks runs the read-only checks. It never writes to $HOME, never
// creates the prefix and never downloads anything: Proton and the launcher are
// acquired only after the user confirms the summary (see AcquireRuntime).
// Every problem found is reported together, so the user fixes them in one go.
func RunPrechecks(opts PrecheckOptions, logger *core.Logger) (*PrecheckResult, error) {
	return runPrechecksWith(opts, logger, defaultPrecheckHost)
}

func runPrechecksWith(opts PrecheckOptions, logger *core.Logger, host precheckHost) (*PrecheckResult, error) {
	logger.Info("Starting precheck phase...")
	fmt.Println()

	gpuCaps, err := host.DetectGPU()
	if err != nil {
		return nil, err
	}
	gpuType := string(gpuCaps.Vendor)
	if gpuCaps.Vendor == core.GPUUnknown {
		gpuType = "Unknown"
	}
	isAMD := gpuCaps.Vendor == core.GPUAMD
	// RDNA4 needs the Proton driver component for the game's native FSR4.
	useFSR41 := gpuCaps.Vendor == core.GPUAMD && gpuCaps.Generation == "RDNA4" && !gpuCaps.Ambiguous
	logger.Info(fmt.Sprintf("GPU Vendor: %s (generation: %s, ambiguous: %t)", gpuType, gpuCaps.Generation, gpuCaps.Ambiguous))

	wineprefix, replaceIncomplete, err := validateWINEPREFIXWith(opts.Wineprefix, logger, host.Files, host.Ask)
	if err != nil {
		return nil, err
	}

	var problems []string

	var missing []string
	for _, tool := range requiredTools() {
		if DiscoverExecutable(tool, host.Commands) == "" {
			missing = append(missing, tool)
		} else {
			logger.Info("[OK] " + tool + " found")
		}
	}
	if len(missing) > 0 {
		problems = append(problems, MissingDependencyGuidance(DetectHost(host.Files, host.Commands), missing))
	}

	if runtime, err := host.FindEAC(); err != nil {
		problems = append(problems, err.Error())
	} else if digest, known, err := host.VerifyEAC(runtime.Path, config.DefaultVersions.EACRuntimeSHA256Allowlist); err != nil {
		problems = append(problems, fmt.Sprintf("The Proton EasyAntiCheat Runtime at %q is incomplete (%v). In Steam, verify the integrity of \"Proton EasyAntiCheat Runtime\", or reinstall it with: %s", runtime.Path, err, eacRuntimeInstallCmd))
	} else {
		logger.Info(fmt.Sprintf("[OK] Proton EasyAntiCheat Runtime found: %s", runtime.Path))
		if runtime.FromEnv {
			logger.Warn("Using PROTON_EAC_RUNTIME from the environment; it is not checked against a Steam install.")
		}
		if !known {
			logger.Warn(fmt.Sprintf("EAC runtime digest %s is not in the known list; Steam has probably updated it. Continuing.", digest))
		}
	}

	protonVer := config.DefaultVersions.ProtonVer
	protonPath := host.ProtonDir(protonVer)
	if config.DefaultVersions.ProtonSHA256 == "" {
		problems = append(problems, "approved Proton SHA-256 pin is required")
	}
	problems = append(problems, checkFreeSpace(wineprefix, protonPath, host)...)

	if opts.LauncherInstaller != "" {
		if _, err := host.Files.Stat(opts.LauncherInstaller); err != nil {
			problems = append(problems, fmt.Sprintf("Launcher installer not found at %s", opts.LauncherInstaller))
		}
	}

	if len(problems) > 0 {
		logger.Error(fmt.Sprintf("Found %d problem(s) to fix before installing:", len(problems)))
		for i, problem := range problems {
			logger.Error(fmt.Sprintf("  %d. %s", i+1, problem))
		}
		return nil, fmt.Errorf("%d precheck problem(s); nothing was changed on this system. Fix them and run the installer again: %s", len(problems), strings.Join(problems, " | "))
	}

	// Verify a user-supplied launcher now, into a private temporary copy, so a
	// bad file fails before confirmation. osslsigncode was checked above.
	var stagedLauncher, launcherTempDir string
	if opts.LauncherInstaller != "" {
		var check packages.LauncherCheck
		stagedLauncher, launcherTempDir, check, err = host.StageLauncher(opts.LauncherInstaller)
		if err != nil {
			return nil, err
		}
		if warning := check.Warning(); warning != "" {
			logger.Warn(warning)
		}
		logger.Info(fmt.Sprintf("[OK] Launcher installer verified: %s", opts.LauncherInstaller))
	}

	logger.Info("[OK] All prechecks passed!")
	fmt.Println()

	return &PrecheckResult{
		WINEPREFIX:        wineprefix,
		ReplaceIncomplete: replaceIncomplete,
		GPUType:           gpuType,
		IsAMDGPU:          isAMD,
		GPUCapabilities:   gpuCaps,
		UseFSR41:          useFSR41,
		ProtonVer:         protonVer,
		ProtonPath:        protonPath,
		LauncherInstaller: stagedLauncher,
		LauncherTempDir:   launcherTempDir,
	}, nil
}

func checkFreeSpace(wineprefix, protonPath string, host precheckHost) []string {
	var problems []string
	check := func(label, path string, need uint64) {
		existing := nearestExistingDir(path, host.Files)
		free, err := host.FreeBytes(existing)
		if err != nil {
			return // Unknown filesystem stats should not block the install.
		}
		if free < need {
			problems = append(problems, fmt.Sprintf("Not enough free space for %s at %s: %.1f GiB free, at least %.0f GiB needed.", label, existing, float64(free)/(1<<30), float64(need)/(1<<30)))
		}
	}
	check("the Bellum prefix", wineprefix, minPrefixFreeBytes)
	if !isDirWith(protonPath, host.Files) {
		check("Proton", protonPath, minProtonFreeBytes)
		check("the Proton download", os.TempDir(), minTempFreeBytes)
	}
	return problems
}

func nearestExistingDir(path string, files FileStore) string {
	for !isDirWith(path, files) && path != "/" && path != "." {
		path = filepath.Dir(path)
	}
	return path
}

func freeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}

// AcquireRuntime downloads, verifies and patches the pinned Proton. It runs
// only after the user has confirmed the install summary.
func AcquireRuntime(result *PrecheckResult, workdir string, logger *core.Logger) error {
	logFile := filepath.Join(workdir, "logs", "installer.log")
	return packages.EnsureProtonWithLog(result.ProtonPath, result.ProtonVer, result.IsAMDGPU, result.UseFSR41, logFile, logger)
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
	return syscall.Access(path, 2 /* W_OK */) == nil
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
