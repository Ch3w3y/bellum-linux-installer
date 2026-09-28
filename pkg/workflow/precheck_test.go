package workflow

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bellum-installer/pkg/core"
	"bellum-installer/pkg/packages"
)

type precheckCommands struct{ available map[string]bool }

func (precheckCommands) Run(core.RunMode, []string, *core.Logger, string) error { return nil }
func (precheckCommands) Output([]string) (string, error) {
	return "", errors.New("no host commands in tests")
}
func (c precheckCommands) LookPath(name string) string {
	if c.available[name] {
		return "/usr/bin/" + name
	}
	return ""
}

// fedoraFiles serves a Fedora os-release and passes everything else to disk.
type fedoraFiles struct{ osFiles }

func (f fedoraFiles) ReadFile(path string) ([]byte, error) {
	if path == "/etc/os-release" {
		return []byte("ID=fedora\n"), nil
	}
	return f.osFiles.ReadFile(path)
}

func newPrecheckFixture(t *testing.T, tools ...string) (string, precheckHost, PrecheckOptions) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	runtime := filepath.Join(t.TempDir(), "eac")
	if err := os.Mkdir(runtime, 0700); err != nil {
		t.Fatal(err)
	}
	available := map[string]bool{"dnf": true}
	for _, tool := range tools {
		available[tool] = true
	}
	host := precheckHost{
		Commands:  precheckCommands{available: available},
		Files:     fedoraFiles{},
		DetectGPU: func() (core.GPUCapabilities, error) { return core.GPUCapabilities{Vendor: core.GPUAMD}, nil },
		Ask:       func(string) bool { return true },
		FreeBytes: func(string) (uint64, error) { return 100 << 30, nil },
		VerifyEAC: func(string, []string) (string, bool, error) { return "digest", true, nil },
		StageLauncher: func(string) (string, string, packages.LauncherCheck, error) {
			return "", "", packages.LauncherCheck{}, errors.New("unexpected launcher staging")
		},
		ProtonDir: func(v string) string { return filepath.Join(home, ".local", "share", "bellum", "proton", v) },
		FindEAC:   func() (EACRuntime, error) { return EACRuntime{Path: runtime}, nil },
	}
	opts := PrecheckOptions{Wineprefix: filepath.Join(t.TempDir(), "Bellum"), Workdir: t.TempDir()}
	return home, host, opts
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	var written []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && path != dir {
			written = append(written, path)
		}
		return nil
	})
	if len(written) > 0 {
		t.Fatalf("prechecks wrote to %s: %v", dir, written)
	}
}

func TestPrechecksReportEveryMissingToolAtOnce(t *testing.T) {
	home, host, opts := newPrecheckFixture(t, "flock")
	host.FindEAC = func() (EACRuntime, error) { return findEACRuntime(home, "", host.Files) }
	logger, _ := core.NewLogger("")
	_, err := runPrechecksWith(opts, logger, host)
	if err == nil {
		t.Fatal("expected prechecks to fail")
	}
	msg := err.Error()
	for _, want := range []string{"2 precheck problem(s)", "python3", "dnf install python3", "steam://install/1826330", "nothing was changed"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error lacks %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "wine ") {
		t.Errorf("system Wine must no longer be required: %s", msg)
	}
	assertEmptyDir(t, home)
	if _, err := os.Lstat(opts.Wineprefix); !os.IsNotExist(err) {
		t.Fatalf("prechecks created the prefix: %v", err)
	}
}

func TestPrechecksAreReadOnlyAndIndependentOfWorkingDirectory(t *testing.T) {
	home, host, opts := newPrecheckFixture(t, "python3", "flock")
	elsewhere := t.TempDir()
	t.Chdir(elsewhere)
	logger, _ := core.NewLogger("")
	result, err := runPrechecksWith(opts, logger, host)
	if err != nil {
		t.Fatalf("prechecks failed: %v", err)
	}
	if result.WINEPREFIX != opts.Wineprefix || result.ProtonPath == "" || result.ReplaceIncomplete {
		t.Fatalf("unexpected result: %+v", result)
	}
	assertEmptyDir(t, home)
	assertEmptyDir(t, elsewhere)
	if _, err := os.Lstat(opts.Wineprefix); !os.IsNotExist(err) {
		t.Fatalf("prechecks created the prefix: %v", err)
	}
}

func TestPrechecksRejectLowDiskSpace(t *testing.T) {
	_, host, opts := newPrecheckFixture(t, "python3", "flock")
	host.FreeBytes = func(string) (uint64, error) { return 1 << 30, nil }
	logger, _ := core.NewLogger("")
	_, err := runPrechecksWith(opts, logger, host)
	if err == nil || !strings.Contains(err.Error(), "Not enough free space") {
		t.Fatalf("expected a disk space problem, got %v", err)
	}
}

func TestPrechecksOfferToInstallEACThroughSteam(t *testing.T) {
	home, host, opts := newPrecheckFixture(t, "python3", "flock", "steam")
	host.FindEAC = func() (EACRuntime, error) { return findEACRuntime(home, "", host.Files) }
	var requested []string
	host.RequestEAC = func(argv []string) error {
		requested = argv
		return nil
	}
	polls := 0
	host.Sleep = func(time.Duration) {
		// Steam finishes the install after a few polls.
		if polls++; polls == 3 {
			installEAC(t, filepath.Join(home, ".local", "share", "Steam"), "4")
		}
	}
	logger, _ := core.NewLogger("")
	if _, err := runPrechecksWith(opts, logger, host); err != nil {
		t.Fatalf("prechecks failed: %v", err)
	}
	if strings.Join(requested, " ") != "steam steam://install/1826330" {
		t.Fatalf("requested %q", requested)
	}

	// Without Steam there is nothing to ask; the problem is reported instead.
	home2, host2, opts2 := newPrecheckFixture(t, "python3", "flock")
	host2.FindEAC = func() (EACRuntime, error) { return findEACRuntime(home2, "", host2.Files) }
	host2.RequestEAC = func([]string) error { t.Fatal("Steam requested without Steam"); return nil }
	if _, err := runPrechecksWith(opts2, logger, host2); err == nil || !strings.Contains(err.Error(), "steam://install/1826330") {
		t.Fatalf("expected the EAC problem, got %v", err)
	}
}

func TestPrechecksCarryTheDetectedPlatform(t *testing.T) {
	_, host, opts := newPrecheckFixture(t, "python3", "flock")
	detected := core.Platform{
		OS:       core.OSInfo{ID: "steamos", VersionID: "3.7", Family: core.OSArch},
		Hardware: core.HardwareInfo{SKU: core.SKUDeckOLED},
		GPU:      core.GPUCapabilities{Vendor: core.GPUAMD, Generation: "RDNA2", FSR: true},
		Session:  core.SessionInfo{Kind: core.SessionGamescope, GameMode: core.TriYes},
	}
	host.DetectGPU = func() (core.GPUCapabilities, error) { t.Fatal("GPU probed twice"); return core.GPUCapabilities{}, nil }
	host.DetectPlatform = func() (core.Platform, error) { return detected, nil }
	logger, _ := core.NewLogger("")
	result, err := runPrechecksWith(opts, logger, host)
	if err != nil {
		t.Fatal(err)
	}
	if result.Platform.Hardware.SKU != core.SKUDeckOLED || result.GPUCapabilities.Generation != "RDNA2" || result.UseFSR41 || !result.IsAMDGPU {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := ProfileSummary(result.Platform); got != "Steam Deck OLED · SteamOS 3 · RDNA2 · Game Mode (expected)" {
		t.Fatalf("profile summary %q", got)
	}

	// A platform probe failure is reported, like a GPU probe failure was.
	host.DetectPlatform = func() (core.Platform, error) { return core.Platform{}, errors.New("probe failed") }
	if _, err := runPrechecksWith(opts, logger, host); err == nil {
		t.Fatal("expected the probe error")
	}
}

func TestSteamOSLocationWarning(t *testing.T) {
	steamos := core.Platform{OS: core.OSInfo{ID: "steamos"}}
	for prefix, warn := range map[string]bool{
		"/home/deck/Games/Bellum":         false,
		"/run/media/deck/SD/Bellum":       false,
		"/opt/Bellum":                     true,
		"/home/deckhand/Bellum":           true,
		"/home/deck/../other/Bellum":      true,
		"/run/media-not-really/sd/Bellum": true,
	} {
		if got := steamOSLocationWarning(filepath.Clean(prefix), "/home/deck", steamos) != ""; got != warn {
			t.Errorf("%s: warning=%t, want %t", prefix, got, warn)
		}
	}
	if steamOSLocationWarning("/opt/Bellum", "/home/deck", core.Platform{OS: core.OSInfo{ID: "arch"}}) != "" {
		t.Error("only SteamOS resets the system on update")
	}
}

// cardTypeFiles serves /sys/block/mmcblk0/device/type.
type cardTypeFiles struct {
	osFiles
	cardType string
}

func (f cardTypeFiles) ReadFile(path string) ([]byte, error) {
	if path == "/sys/block/mmcblk0/device/type" && f.cardType != "" {
		return []byte(f.cardType + "\n"), nil
	}
	return nil, os.ErrNotExist
}

func TestIsSDCard(t *testing.T) {
	const header = "Filesystem 1024-blocks Used Available Capacity Mounted on\n"
	for _, tc := range []struct {
		df, cardType string
		want         bool
	}{
		{header + "/dev/mmcblk0p1 500000 1 499999 1% /run/media/deck/SD\n", "SD", true},
		{header + "/dev/mmcblk0 500000 1 499999 1% /run/media/deck/SD\n", "SD", true},
		// Internal eMMC is also mmcblk.
		{header + "/dev/mmcblk0p8 500000 1 499999 1% /home\n", "MMC", false},
		{header + "/dev/mmcblk0p1 500000 1 499999 1% /x\n", "", false},
		{header + "/dev/nvme0n1p8 500000 1 499999 1% /home\n", "SD", false},
		{header + "/dev/mmcblk0p1/../../etc 1 1 1 1% /x\n", "SD", false},
		{"", "SD", false},
	} {
		if got := isSDCardWith("/x", fakeCommands{output: tc.df}, cardTypeFiles{cardType: tc.cardType}); got != tc.want {
			t.Errorf("%q (%s): got %t, want %t", tc.df, tc.cardType, got, tc.want)
		}
	}
}

func TestPackageFamilyCoversDerivatives(t *testing.T) {
	for id, want := range map[string]string{
		"cachyos": "arch", "endeavouros": "arch", "nobara": "fedora", "pop": "debian",
		"linuxmint": "debian", "opensuse-tumbleweed": "opensuse", "gentoo": "unknown",
	} {
		if got := (Host{ID: id}).PackageFamily(); got != want {
			t.Errorf("%s: got %s, want %s", id, got, want)
		}
	}
	if got := (Host{ID: "zorin", IDLike: "ubuntu debian"}).PackageFamily(); got != "debian" {
		t.Errorf("ID_LIKE fallback: %s", got)
	}
}
