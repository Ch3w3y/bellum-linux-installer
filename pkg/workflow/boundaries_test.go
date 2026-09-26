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

func TestUMURunPrecheckUsesInjectedHost(t *testing.T) {
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkUMURun(fakeCommands{found: "/fake/bin/umu-run"}, logger); err != nil {
		t.Fatal(err)
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

func TestRDNA4EnablesOnlyNativeFSR4DriverComponent(t *testing.T) {
	files := fakeFileStore{written: map[string][]byte{}}
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	if err := createLaunchVarsFileAMD("/prefix", "/proton", true, logger, files); err != nil {
		t.Fatal(err)
	}
	content := string(files.written["/prefix/launch_vars.env"])
	if !strings.Contains(content, `PROTON_FSR4_UPGRADE="1"`) || !strings.Contains(content, `PROTON_FSR4_RDNA3_UPGRADE="0"`) {
		t.Fatalf("wrong RDNA4 defaults: %s", content)
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
		"export PROTON_FSR4_UPGRADE=0\n",
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
