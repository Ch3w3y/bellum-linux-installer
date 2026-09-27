package workflow

import (
	"bellum-installer/pkg/core"
	"os"
	"strings"
	"testing"
)

type fakeCommands struct {
	found  string
	output string
}

func (f fakeCommands) Run(_ core.RunMode, _ []string, _ *core.Logger, _ string) error { return nil }
func (f fakeCommands) Output(_ []string) (string, error)                              { return f.output, nil }
func (f fakeCommands) LookPath(name string) string                                    { return f.found }

func TestDiscoverExecutableUsesInjectedHost(t *testing.T) {
	got := DiscoverExecutable("wine", fakeCommands{found: "/fake/bin/wine"})
	if got != "/fake/bin/wine" {
		t.Fatalf("got %q", got)
	}
}

type releaseFiles struct {
	fakeFileStore
	release string
}

func (f releaseFiles) ReadFile(string) ([]byte, error) { return []byte(f.release), nil }

type namedCommands struct{ available map[string]string }

func (f namedCommands) Run(_ core.RunMode, _ []string, _ *core.Logger, _ string) error { return nil }
func (f namedCommands) Output(_ []string) (string, error)                              { return "", nil }
func (f namedCommands) LookPath(n string) string {
	if n == "" {
		return ""
	}
	return f.available[n]
}

func TestHostDiscoveryAndDependencyGuidance(t *testing.T) {
	tests := []struct {
		name, release, manager, family, command string
		immutable                               bool
	}{
		{"arch", "ID=arch\n", "pacman", "arch", "pacman -S", false},
		{"fedora", "ID=fedora\n", "dnf", "fedora", "dnf install", false},
		{"ubuntu", "ID=ubuntu\n", "apt-get", "debian", "apt install", false},
		{"opensuse", "ID=opensuse-tumbleweed\n", "zypper", "opensuse", "zypper install", false},
		{"steamos", "ID=steamos\n", "pacman", "unknown", "immutable", true},
		{"bazzite", "ID=bazzite\n", "", "unknown", "immutable", true},
		{"unknown", "ID=void\n", "", "unknown", "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := releaseFiles{release: tt.release}
			commands := namedCommands{available: map[string]string{tt.manager: "/usr/bin/" + tt.manager}}
			host := DetectHost(files, commands)
			if host.PackageFamily() != tt.family || host.Immutable != tt.immutable {
				t.Fatalf("host = %#v family=%s", host, host.PackageFamily())
			}
			if tt.manager != "" && host.PackageManager != tt.manager {
				t.Fatalf("manager = %q", host.PackageManager)
			}
			msg := MissingDependencyGuidance(host, []string{"wine", "umu-run"})
			if !strings.Contains(msg, tt.command) || !strings.Contains(msg, "wine") {
				t.Fatalf("guidance: %s", msg)
			}
		})
	}
}

func TestUMURunPrecheckUsesInjectedHost(t *testing.T) {
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkUMURun(fakeCommands{found: "/fake/bin/umu-run"}, logger); err != nil {
		t.Fatal(err)
	}
}

func TestLauncherPrecheckRequiresOsslsigncodeForDownloadPath(t *testing.T) {
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	err = checkLauncherInstaller("", logger, fakeFileStore{}, namedCommands{available: map[string]string{"wget": "/usr/bin/wget"}})
	if err == nil || !strings.Contains(err.Error(), "osslsigncode") {
		t.Fatalf("expected missing osslsigncode precheck error, got %v", err)
	}
}

func TestSSDPrecheckUsesInjectedCommandOutput(t *testing.T) {
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	if !isSSDWith("/mnt/bellum", logger, fakeCommands{output: "0\n"}) {
		t.Fatal("expected injected non-rotational device result to be treated as SSD")
	}
}

type fakeFileStore struct{ written map[string][]byte }

func (f fakeFileStore) Stat(string) (os.FileInfo, error)      { return nil, os.ErrNotExist }
func (f fakeFileStore) ReadDir(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist }
func (f fakeFileStore) MkdirAll(string, os.FileMode) error    { return nil }
func (f fakeFileStore) RemoveAll(string) error                { return nil }
func (f fakeFileStore) Remove(string) error                   { return nil }
func (f fakeFileStore) ReadFile(string) ([]byte, error)       { return nil, os.ErrNotExist }
func (f fakeFileStore) WriteFile(path string, data []byte, _ os.FileMode) error {
	f.written[path] = append([]byte(nil), data...)
	return nil
}

func TestConfigurationWritesLaunchVarsThroughFileBoundary(t *testing.T) {
	files := fakeFileStore{written: map[string][]byte{}}
	boundaries := DefaultBoundaries
	boundaries.Commands = fakeCommands{}
	boundaries.MutatePrefix = nil
	boundaries.Files = files
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	if err := RunConfigurationWithBoundaries(ConfigureConfig{WINEPREFIX: "/prefix", ProtonPath: "/proton", GPUType: "AMD", GPUCapabilities: core.GPUCapabilities{Vendor: core.GPUAMD, FSR: true}}, logger, boundaries); err != nil {
		t.Fatal(err)
	}
	content, ok := files.written["/prefix/launch_vars.env"]
	if !ok {
		t.Fatal("launch variables did not pass through injected FileStore")
	}
	if !strings.Contains(string(content), `export WINEPREFIX='/prefix'`) || !strings.Contains(string(content), "PROTON_EAC_RUNTIME") {
		t.Fatalf("unexpected launch vars: %s", content)
	}
}

func TestRDNA4RequestsFSR4AndDoesNotUseRetiredRDNA3Switch(t *testing.T) {
	files := fakeFileStore{written: map[string][]byte{}}
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	if err := createLaunchVarsFileAMD("/prefix", "/proton", true, logger, files); err != nil {
		t.Fatal(err)
	}
	content := string(files.written["/prefix/launch_vars.env"])
	if !strings.Contains(content, `PROTON_FSR4_UPGRADE="1"`) || strings.Contains(content, `PROTON_FSR4_RDNA3_UPGRADE`) {
		t.Fatalf("wrong RDNA4 defaults: %s", content)
	}
}

func TestRDNA3DoesNotClaimRuntimeAutoStagingIsDisabled(t *testing.T) {
	files := fakeFileStore{written: map[string][]byte{}}
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	if err := createLaunchVarsFileAMD("/prefix", "/proton", false, logger, files); err != nil {
		t.Fatal(err)
	}
	content := string(files.written["/prefix/launch_vars.env"])
	if strings.Contains(content, "PROTON_FSR4_UPGRADE") || strings.Contains(content, "PROTON_FSR4_RDNA3_UPGRADE") {
		t.Fatalf("RDNA3 environment must not claim to disable the runtime-staged DLL: %s", content)
	}
}

func TestGenericLaunchVarsAreSourceable(t *testing.T) {
	files := fakeFileStore{written: map[string][]byte{}}
	if err := createLaunchVarsFileGeneric("/prefix with spaces", "/proton's build", files); err != nil {
		t.Fatal(err)
	}
	content := string(files.written["/prefix with spaces/launch_vars.env"])
	if strings.Contains(content, `\n`) {
		t.Fatalf("launch variables contain literal newline escapes: %q", content)
	}
	for _, line := range []string{
		"export PROTONPATH='/proton'\"'\"'s build'\n",
		"export WINEPREFIX='/prefix with spaces'\n",
		"export PROTON_EAC_RUNTIME=",
		"export PROTON_DLSS_UPGRADE=0\n",
	} {
		if !strings.Contains(content, line) {
			t.Fatalf("launch variables missing %q: %q", line, content)
		}
	}
}

func TestFileBoundaryRejectsWritesUnderGameInstallTree(t *testing.T) {
	files := fakeFileStore{written: map[string][]byte{}}
	guard := GuardGameTreeWrites{FileStore: files, GameInstallDir: "/prefix/drive_c/Program Files/Astarte Industries/Bellum/Project_Bellum"}
	err := guard.WriteFile("/prefix/drive_c/Program Files/Astarte Industries/Bellum/Project_Bellum/Binaries/Win64/D3D12/x64/D3D12Core.dll", []byte("blocked"), 0644)
	if err == nil {
		t.Fatal("expected game install tree write to be rejected")
	}
	if len(files.written) != 0 {
		t.Fatalf("underlying store received rejected write: %#v", files.written)
	}
	if err := guard.WriteFile("/prefix/launch_vars.env", []byte("allowed"), 0644); err != nil {
		t.Fatalf("prefix write should remain allowed: %v", err)
	}
}

func TestDependencyGuidanceIncludesGlxinfoPackage(t *testing.T) {
	cases := map[string]string{
		"arch":     "mesa-demos",
		"fedora":   "glx-utils",
		"debian":   "mesa-utils",
		"opensuse": "Mesa-demo-x",
	}
	for release, wantPkg := range map[string]string{
		"ID=arch\n":                cases["arch"],
		"ID=fedora\n":              cases["fedora"],
		"ID=ubuntu\n":              cases["debian"],
		"ID=opensuse-tumbleweed\n": cases["opensuse"],
	} {
		host := DetectHost(releaseFiles{release: release}, namedCommands{})
		msg := MissingDependencyGuidance(host, []string{"glxinfo"})
		if !strings.Contains(msg, wantPkg) {
			t.Fatalf("%s guidance missing %s: %s", host.PackageFamily(), wantPkg, msg)
		}
	}
}
