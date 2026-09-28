package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
	"bellum-installer/pkg/gui"
	"bellum-installer/pkg/packages"
)

// PrecheckResult holds the result of precheck validation
type PrecheckResult struct {
	WINEPREFIX        string
	ReplaceIncomplete bool
	// Update is set when WINEPREFIX already holds a finished install that
	// the user chose to update to the current pins.
	Update            bool
	LauncherInstaller string
	LauncherTempDir   string
	GPUType           string
	IsAMDGPU          bool
	GPUCapabilities   core.GPUCapabilities
	UseFSR41          bool
	// Platform is the detected platform snapshot (step 2); its GPU matches
	// GPUCapabilities.
	Platform   core.Platform
	ProtonVer  string
	ProtonPath string
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
	// The path ends up in desktop entries and registry commands, where a
	// newline or other control character would inject extra fields.
	if strings.IndexFunc(input, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("the install location contains a control character")
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
	path, replace, _, err := validateWINEPREFIXWith(wineprefix, logger, DefaultBoundaries.Files, core.AskBool)
	return path, replace, err
}

func validateWINEPREFIXWith(wineprefix string, logger *core.Logger, files FileStore, ask func(string) bool) (string, bool, bool, error) {
	if wineprefix == "" || !filepath.IsAbs(wineprefix) || filepath.Base(wineprefix) != "Bellum" {
		return "", false, false, fmt.Errorf("WINEPREFIX must be an absolute path ending in Bellum: %q", wineprefix)
	}
	wineprefix = filepath.Clean(wineprefix)
	logger.Info(fmt.Sprintf("Install folder: %s", core.Colorize(wineprefix, core.ColorBoldYellow)))

	// Find the nearest existing ancestor; the prefix is created beneath it later.
	parent := wineprefix
	for !isDirWith(parent, files) && parent != "/" {
		parent = filepath.Dir(parent)
	}
	if !isDirWith(parent, files) {
		return "", false, false, fmt.Errorf("WINEPREFIX path is not on a valid mounted filesystem: %s", wineprefix)
	}

	replaceIncomplete := false
	state, err := inspectPrefix(wineprefix, files)
	if err != nil {
		return "", false, false, fmt.Errorf("inspect %s: %w", wineprefix, err)
	}
	switch state {
	case prefixIncomplete:
		logger.Warn(fmt.Sprintf("A previous Bellum install at %s didn't finish.", wineprefix))
		if !ask("Start over? Everything in that folder will be replaced. (Y/n): ") {
			return "", false, false, fmt.Errorf("installation cancelled: the unfinished install at %s was kept", wineprefix)
		}
		replaceIncomplete = true
	case prefixInstalled:
		logger.Info(fmt.Sprintf("Bellum is already installed at %s.", wineprefix))
		if !ask("Update it to the latest tested Proton and settings? Your game and launcher login are kept. (Y/n): ") {
			return "", false, false, fmt.Errorf("nothing to do: Bellum is already installed at %s. To reinstall from scratch, run the uninstaller first: ./uninstaller --wineprefix %s", wineprefix, wineprefix)
		}
		return wineprefix, false, true, nil
	case prefixForeign:
		return "", false, false, fmt.Errorf("%s already exists and is not an empty folder or a Bellum install. Choose another location or move that folder away", wineprefix)
	}

	if !isWritable(parent) {
		return "", false, false, fmt.Errorf("WINEPREFIX parent directory is not writable: %s", parent)
	}
	logger.Info("[OK] Install folder is writable")

	if isSDCardWith(parent, DefaultBoundaries.Commands, files) {
		// lsblk reports SD cards as non-rotational, so check them first.
		logger.Warn("Install folder is on a microSD card. Bellum works there, but loading is slower than on the internal SSD.")
	} else if isSSD(parent, logger) {
		logger.Info("[OK] Install folder is on an SSD/NVMe drive")
	} else {
		logger.Warn("Install folder is NOT on an SSD/NVMe drive; loading may be slow")
		if !ask("Astarte Developers strongly recommend using NVMe or SSD for the game. Are you sure you want to proceed? (Y/n): ") {
			return "", false, false, fmt.Errorf("installation cancelled by user")
		}
	}

	return wineprefix, replaceIncomplete, false, nil
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

// PrecheckError lists every problem the read-only prechecks found. Each
// problem has already been logged on its own line.
type PrecheckError struct{ Problems []string }

func (e *PrecheckError) Error() string {
	return fmt.Sprintf("%d precheck problem(s); nothing was changed on this system. Fix them and run the installer again: %s", len(e.Problems), strings.Join(e.Problems, " | "))
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
	Commands  CommandRunner
	Files     FileStore
	DetectGPU func() (core.GPUCapabilities, error)
	// DetectPlatform, when set, supplies the whole platform snapshot and its
	// GPU replaces DetectGPU's.
	DetectPlatform func() (core.Platform, error)
	Ask            func(string) bool
	FreeBytes      func(string) (uint64, error)
	VerifyEAC      func(string, []string) (string, bool, error)
	StageLauncher  func(string) (string, string, packages.LauncherCheck, error)
	ProtonDir      func(string) string
	FindEAC        func() (EACRuntime, error)
	// RequestEAC runs the given command to ask Steam to install the EAC
	// runtime; Steam shows its own confirmation.
	RequestEAC func([]string) error
	// Sleep is time.Sleep, injectable for tests.
	Sleep func(time.Duration)
}

// steamInstallCommand returns the command that asks the user's Steam
// (native, Flatpak or Snap) to install the EAC runtime, or nil without Steam.
func steamInstallCommand(commands CommandRunner, files FileStore) []string {
	uri := "steam://install/" + eacRuntimeAppID
	if DiscoverExecutable("steam", commands) != "" {
		return []string{"steam", uri}
	}
	home, _ := os.UserHomeDir()
	if DiscoverExecutable("flatpak", commands) != "" &&
		(isDirWith(filepath.Join(home, ".var", "app", "com.valvesoftware.Steam"), files) || isDirWith("/var/lib/flatpak/app/com.valvesoftware.Steam", files)) {
		return []string{"flatpak", "run", "com.valvesoftware.Steam", uri}
	}
	if DiscoverExecutable("snap", commands) != "" &&
		(isDirWith(filepath.Join(home, "snap", "steam"), files) || isDirWith("/snap/steam", files)) {
		return []string{"snap", "run", "steam", uri}
	}
	return nil
}

// startDetached starts argv without waiting for it, so Steam keeps running.
func startDetached(argv []string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// eacInstallWait bounds how long the installer waits for Steam.
const eacInstallWait = 20 * time.Minute

func waitForEACInstall(host precheckHost, logger *core.Logger) (EACRuntime, error) {
	logger.Info("Steam should now ask you to install \"Proton EasyAntiCheat Runtime\". Confirm it there; the installer carries on as soon as Steam is done.")
	var rt EACRuntime
	var err error
	for waited := time.Duration(0); waited <= eacInstallWait; waited += 5 * time.Second {
		if rt, err = host.FindEAC(); err == nil {
			return rt, nil
		}
		if waited > 0 && waited%(time.Minute) == 0 {
			logger.Info(fmt.Sprintf("Still waiting for Steam (%d min). Press Ctrl+C to stop; running the installer again resumes from here.", waited/time.Minute))
		}
		host.Sleep(5 * time.Second)
	}
	return rt, err
}

var defaultPrecheckHost = precheckHost{
	RequestEAC:     startDetached,
	Sleep:          time.Sleep,
	Commands:       DefaultBoundaries.Commands,
	Files:          DefaultBoundaries.Files,
	DetectGPU:      core.DetectGPUCapabilities,
	DetectPlatform: detectPlatform,
	Ask:            core.AskBool,
	FreeBytes:      freeBytes,
	VerifyEAC:      packages.VerifyEACRuntime,
	StageLauncher:  packages.StageLauncherInstaller,
	ProtonDir:      packages.GetProtonInstallPath,
	FindEAC: func() (EACRuntime, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return EACRuntime{}, err
		}
		return findEACRuntime(home, os.Getenv("PROTON_EAC_RUNTIME"), DefaultBoundaries.Files)
	},
}

// platformDetectionBudget bounds the whole read-only platform detection.
const platformDetectionBudget = 15 * time.Second

// detectPlatform is the production platform probe. Detection problems never
// block the install: a failed or cancelled detection keeps whatever it
// collected, and a missing GPU falls back to the classic GPU probe.
func detectPlatform() (core.Platform, error) {
	ctx, cancel := context.WithTimeout(context.Background(), platformDetectionBudget)
	defer cancel()
	p, err := core.DetectPlatform(ctx)
	if err != nil {
		var partial *core.PartialError
		if errors.As(err, &partial) {
			p = partial.Snapshot
		}
		if p.GPU.Vendor == "" {
			caps, gerr := core.DetectGPUCapabilities()
			if gerr != nil {
				return core.Platform{}, gerr
			}
			p.GPU = caps
		}
	}
	return p, nil
}

// platformFor runs the host's platform detection, or builds a GPU-only
// snapshot from DetectGPU when no platform probe is configured.
func platformFor(host precheckHost) (core.Platform, error) {
	if host.DetectPlatform != nil {
		return host.DetectPlatform()
	}
	caps, err := host.DetectGPU()
	if err != nil {
		return core.Platform{}, err
	}
	return core.Platform{GPU: caps}, nil
}

// steamOSLocationWarning warns when a SteamOS install location is outside
// $HOME and removable media: SteamOS updates replace the rest of the system.
func steamOSLocationWarning(prefix, home string, p core.Platform) string {
	if p.OS.ID != "steamos" || home == "" {
		return ""
	}
	within := func(root string) bool {
		rel, err := filepath.Rel(root, prefix)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
	}
	if within(home) || within("/run/media") {
		return ""
	}
	return fmt.Sprintf("%s is outside your home folder. SteamOS updates replace everything outside /home and removable drives, so install under %s or on a microSD card instead.", prefix, home)
}

// requiredTools lists the host commands the install needs. Everything else
// is provided: Proton and umu-launcher are downloaded and pinned, downloads
// and Authenticode checks are done in Go, and winetricks comes with Proton.
// python3 runs the umu-launcher zipapp; flock (util-linux) guards the game
// wrapper against double launches.
func requiredTools() []string {
	return []string{"python3", "flock"}
}

// pythonTooOld reports whether python3 is older than umu-launcher needs
// (3.10). Unknown versions are not treated as too old.
func pythonTooOld(commands CommandRunner) bool {
	out, err := commands.Output([]string{"python3", "-c", "import sys; print(sys.version_info >= (3, 10))"})
	return err == nil && strings.TrimSpace(out) == "False"
}

// RunPrechecks runs the read-only checks. It never writes to $HOME, never
// creates the prefix and never downloads anything: Proton and the launcher are
// acquired only after the user confirms the summary (see AcquireRuntime).
// Every problem found is reported together, so the user fixes them in one go.
func RunPrechecks(opts PrecheckOptions, logger *core.Logger) (*PrecheckResult, error) {
	return runPrechecksWith(opts, logger, defaultPrecheckHost)
}

func runPrechecksWith(opts PrecheckOptions, logger *core.Logger, host precheckHost) (*PrecheckResult, error) {

	platform, err := platformFor(host)
	if err != nil {
		return nil, err
	}
	gpuCaps := platform.GPU
	if gpuCaps.Vendor == "" {
		gpuCaps.Vendor = core.GPUUnknown
		platform.GPU = gpuCaps
	}
	gpuType := string(gpuCaps.Vendor)
	if gpuCaps.Vendor == core.GPUUnknown {
		gpuType = "Unknown"
	}
	isAMD := gpuCaps.Vendor == core.GPUAMD
	// RDNA4 needs the Proton driver component for the game's native FSR4.
	useFSR41 := core.LaunchProfileFor(gpuCaps).UsesFSR4Upgrade()
	gpuLine := "GPU: " + gpuType
	if gpuCaps.Generation != "" {
		gpuLine += " " + gpuCaps.Generation
	}
	if gpuCaps.Renderer != "" && !strings.HasPrefix(gpuCaps.Renderer, "undetected") {
		gpuLine += core.ColorGrayBold + "  (" + core.SanitizeField(gpuCaps.Renderer, core.MaxRendererLen) + ")" + core.ColorReset
	}
	if gpuCaps.Vendor != core.GPUUnknown {
		gpuLine = "[OK] " + gpuLine
	}
	logger.Info(gpuLine)
	if gpuCaps.Vendor == core.GPUUnknown {
		logger.Warn("Your GPU wasn't recognised (common in VMs and on some hybrid laptops). Bellum will use generic Proton settings without vendor-specific features.")
	}
	logger.Info("Detected: " + ProfileSummary(platform))
	for _, d := range platform.Diagnostics {
		logger.Record(fmt.Sprintf("detection: %s %s %s: %s", d.Code, d.Source, d.Outcome, d.Detail))
	}
	// NVIDIA driver problems warn, never block.
	for _, warning := range NVIDIAWarnings(platform) {
		logger.Warn(warning)
	}

	wineprefix, replaceIncomplete, update, err := validateWINEPREFIXWith(opts.Wineprefix, logger, host.Files, host.Ask)
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()
	if warning := steamOSLocationWarning(wineprefix, home, platform); warning != "" {
		logger.Warn(warning)
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
	} else if pythonTooOld(host.Commands) {
		problems = append(problems, "python3 is older than 3.10, which umu-launcher needs. Update python3 with your distribution's package manager.")
	}

	runtime, eacErr := host.FindEAC()
	if eacErr != nil && host.RequestEAC != nil && os.Getenv("PROTON_EAC_RUNTIME") == "" {
		if argv := steamInstallCommand(host.Commands, host.Files); argv != nil &&
			host.Ask("Bellum needs the free Proton EasyAntiCheat Runtime from Steam. Ask Steam to install it now? (Y/n): ") {
			if err := host.RequestEAC(argv); err != nil {
				logger.Warn(fmt.Sprintf("Couldn't start Steam: %v", err))
			} else {
				runtime, eacErr = waitForEACInstall(host, logger)
			}
		}
	}
	if eacErr != nil {
		problems = append(problems, eacErr.Error())
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
	problems = append(problems, checkFreeSpace(wineprefix, protonPath, !update, host)...)

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
		return nil, &PrecheckError{Problems: problems}
	}

	// Verify a user-supplied launcher now, into a private temporary copy, so a
	// bad file fails before confirmation.
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

	logger.Info("[OK] Your system is ready")

	return &PrecheckResult{
		WINEPREFIX:        wineprefix,
		ReplaceIncomplete: replaceIncomplete,
		Update:            update,
		GPUType:           gpuType,
		IsAMDGPU:          isAMD,
		GPUCapabilities:   gpuCaps,
		UseFSR41:          useFSR41,
		Platform:          platform,
		ProtonVer:         protonVer,
		ProtonPath:        protonPath,
		LauncherInstaller: stagedLauncher,
		LauncherTempDir:   launcherTempDir,
	}, nil
}

func checkFreeSpace(wineprefix, protonPath string, newPrefix bool, host precheckHost) []string {
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
	if newPrefix {
		check("the Bellum prefix", wineprefix, minPrefixFreeBytes)
	}
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

// AcquireRuntime downloads and verifies the pinned umu-launcher and Proton.
// It runs only after the user has confirmed the install summary.
func AcquireRuntime(result *PrecheckResult, workdir string, logger *core.Logger) error {
	umu, err := packages.EnsureUMU(packages.UMUInstallDir(config.DefaultVersions.UMUVersion), logger)
	if err != nil {
		return err
	}
	umuRunBinary = umu
	logFile := filepath.Join(workdir, "logs", "installer.log")
	return packages.EnsureProtonWithLog(result.ProtonPath, result.ProtonVer, logFile, logger)
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
	deviceName := filepath.Base(dfDevice(path, commands))
	return strings.HasPrefix(deviceName, "nvme") || strings.HasPrefix(deviceName, "sd") || strings.HasPrefix(deviceName, "vd")
}

// dfDevice returns the device backing path according to `df -P`, or "".
func dfDevice(path string, commands CommandRunner) string {
	out, err := commands.Output([]string{"df", "-P", path})
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return ""
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

var mmcPartitionRe = regexp.MustCompile(`^(mmcblk[0-9]+)(?:p[0-9]+)?$`)

// isSDCardWith reports whether path is on an SD card (the Steam Deck's
// microSD slot). Internal eMMC storage is also an mmcblk device, so the
// kernel's card type decides: "SD" for SD cards, "MMC" for eMMC.
func isSDCardWith(path string, commands CommandRunner, files FileStore) bool {
	m := mmcPartitionRe.FindStringSubmatch(filepath.Base(dfDevice(path, commands)))
	if m == nil {
		return false
	}
	data, err := files.ReadFile(filepath.Join("/sys/block", m[1], "device", "type"))
	return err == nil && strings.TrimSpace(string(data)) == "SD"
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
