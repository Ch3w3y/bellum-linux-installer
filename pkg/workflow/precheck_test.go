package workflow

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	home, host, opts := newPrecheckFixture(t, "wget")
	host.FindEAC = func() (EACRuntime, error) { return findEACRuntime(home, "", host.Files) }
	logger, _ := core.NewLogger("")
	_, err := runPrechecksWith(opts, logger, host)
	if err == nil {
		t.Fatal("expected prechecks to fail")
	}
	msg := err.Error()
	for _, want := range []string{"2 precheck problem(s)", "umu-run", "osslsigncode", "dnf install osslsigncode", "steam://install/1826330", "nothing was changed"} {
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
	home, host, opts := newPrecheckFixture(t, "umu-run", "osslsigncode", "wget")
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
	_, host, opts := newPrecheckFixture(t, "umu-run", "osslsigncode", "wget")
	host.FreeBytes = func(string) (uint64, error) { return 1 << 30, nil }
	logger, _ := core.NewLogger("")
	_, err := runPrechecksWith(opts, logger, host)
	if err == nil || !strings.Contains(err.Error(), "Not enough free space") {
		t.Fatalf("expected a disk space problem, got %v", err)
	}
}
