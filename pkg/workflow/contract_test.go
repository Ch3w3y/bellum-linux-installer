package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bellum-installer/pkg/core"
)

// Every PROTON_* variable any configuration writes must be in the contract,
// so the pin updater verifies that the next Proton still reads it.
func TestLaunchVarsOnlyUseContractedProtonVariables(t *testing.T) {
	contracted := map[string]bool{"PROTON_EAC_RUNTIME": true}
	for _, r := range ProtonContract() {
		if strings.HasPrefix(r.Needle, "PROTON_") {
			contracted[r.Needle] = true
		}
	}
	logger, _ := core.NewLogger("")
	for _, caps := range []core.GPUCapabilities{rtx, rdna4, {Vendor: core.GPUAMD, Generation: "RDNA3"}, {Vendor: core.GPUIntel}, {Vendor: core.GPUUnknown}} {
		files := fakeFileStore{written: map[string][]byte{}}
		boundaries := WorkflowBoundaries{Commands: fakeCommands{}, Files: files, MutatePrefix: func(core.RunMode, []string, *core.Logger, string) error { return nil }}
		if err := RunConfigurationWithBoundaries(ConfigureConfig{WINEPREFIX: "/prefix", ProtonPath: "/proton", GPUCapabilities: caps, IsFSR41: caps.FSR41}, logger, boundaries); err != nil {
			t.Fatal(err)
		}
		for _, v := range protonVarsIn(string(files.written["/prefix/launch_vars.env"])) {
			if !contracted[v] {
				t.Errorf("%+v writes %s, which ProtonContract doesn't check", caps.Vendor, v)
			}
		}
	}
}

// TestProtonContractAgainstTree checks an extracted Proton tree. It runs when
// BELLUM_PROTON_DIR points at one; tools/pinupdate does the same in CI.
func TestProtonContractAgainstTree(t *testing.T) {
	dir := os.Getenv("BELLUM_PROTON_DIR")
	if dir == "" {
		t.Skip("set BELLUM_PROTON_DIR to an extracted Proton tree")
	}
	cache := map[string]string{}
	for _, r := range ProtonContract() {
		if _, ok := cache[r.Path]; !ok {
			b, err := os.ReadFile(filepath.Join(dir, r.Path))
			if err != nil {
				t.Errorf("%s: %v", r.Path, err)
			}
			cache[r.Path] = string(b)
		}
		if !strings.Contains(cache[r.Path], r.Needle) {
			t.Errorf("%s lacks %q (%s)", r.Path, r.Needle, r.Why)
		}
	}
}
