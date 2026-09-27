package workflow

import (
	"path/filepath"
	"strings"
	"testing"

	"bellum-installer/pkg/core"
)

func scripted(answers ...string) func(string, string) string {
	return func(_ string, def string) string {
		if len(answers) == 0 {
			return def
		}
		a := answers[0]
		answers = answers[1:]
		if a == "" {
			return def
		}
		return a
	}
}

var rtx = core.GPUCapabilities{Vendor: core.GPUNVIDIA, NVAPI: true, DLSS: true}
var rdna4 = core.GPUCapabilities{Vendor: core.GPUAMD, Generation: "RDNA4", FSR41: true}

func TestEnterThroughDefaultsGivesStableWithoutExtras(t *testing.T) {
	logger, _ := core.NewLogger("")
	all := precheckCommands{available: map[string]bool{"mangohud": true, "gamemoderun": true, "gamescope": true}}
	got := chooseInstallOptionsWith(rtx, logger, scripted(), all)
	if got != DefaultInstallOptions() || got.Summary() != "Stable, no extras" {
		t.Fatalf("defaults = %+v (%s)", got, got.Summary())
	}
}

func TestPerformancePresetAndExtras(t *testing.T) {
	logger, _ := core.NewLogger("")
	all := precheckCommands{available: map[string]bool{"mangohud": true, "gamemoderun": true, "gamescope": true}}
	got := chooseInstallOptionsWith(rdna4, logger, scripted("oops", "2", "y", "yes", "n"), all)
	want := InstallOptions{Preset: PresetPerformance, MangoHud: true, GameMode: true}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got.Summary() != "Performance + MangoHud, GameMode" {
		t.Fatalf("summary %q", got.Summary())
	}
}

func TestExtrasAreOnlyOfferedWhenInstalled(t *testing.T) {
	logger, _ := core.NewLogger("")
	asked := 0
	prompt := func(p, def string) string {
		asked++
		return "y"
	}
	got := chooseInstallOptionsWith(core.GPUCapabilities{Vendor: core.GPUIntel}, logger, prompt, precheckCommands{})
	if asked != 0 || got != DefaultInstallOptions() {
		t.Fatalf("asked %d questions, got %+v", asked, got)
	}
}

func TestPerformanceFeaturesPerVendor(t *testing.T) {
	if f := PerformanceFeatures(rtx); len(f) != 1 || !strings.Contains(f[0], "DLSS") {
		t.Fatalf("RTX: %v", f)
	}
	if f := PerformanceFeatures(rdna4); len(f) != 1 || !strings.Contains(f[0], "PROTON_FSR4_UPGRADE") {
		t.Fatalf("RDNA4: %v", f)
	}
	for _, caps := range []core.GPUCapabilities{
		{Vendor: core.GPUAMD, Generation: "RDNA3"},
		{Vendor: core.GPUNVIDIA},
		{Vendor: core.GPUIntel},
		{Vendor: core.GPUUnknown},
	} {
		if f := PerformanceFeatures(caps); len(f) != 0 {
			t.Fatalf("%+v should have no performance extras: %v", caps, f)
		}
	}
}

func TestLaunchVarsFollowPreset(t *testing.T) {
	logger, _ := core.NewLogger("")
	for _, tc := range []struct {
		name   string
		caps   core.GPUCapabilities
		preset Preset
		want   []string
	}{
		{"rtx stable", rtx, PresetStable, []string{`PROTON_ENABLE_NVAPI="0"`, `PROTON_NVIDIA_LIBS="0"`}},
		{"rtx performance", rtx, PresetPerformance, []string{`PROTON_ENABLE_NVAPI="1"`, `PROTON_NVIDIA_LIBS="1"`, `DXVK_ENABLE_NVAPI="1"`}},
		{"rdna4 stable", rdna4, PresetStable, []string{`PROTON_FSR4_UPGRADE="0"`}},
		{"rdna4 performance", rdna4, PresetPerformance, []string{`PROTON_FSR4_UPGRADE="1"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fakeFileStore{written: map[string][]byte{}}
			boundaries := WorkflowBoundaries{Commands: fakeCommands{}, Files: files, MutatePrefix: func(core.RunMode, []string, *core.Logger, string) error { return nil }}
			config := ConfigureConfig{WINEPREFIX: "/prefix", ProtonPath: "/proton", GPUCapabilities: tc.caps, IsFSR41: tc.caps.FSR41,
				Options: InstallOptions{Preset: tc.preset, GameMode: true}}
			if err := RunConfigurationWithBoundaries(config, logger, boundaries); err != nil {
				t.Fatal(err)
			}
			content := string(files.written["/prefix/launch_vars.env"])
			for _, want := range append(tc.want, `BELLUM_GAMEMODE="1"`, `BELLUM_MANGOHUD="0"`) {
				if !strings.Contains(content, want) {
					t.Errorf("launch_vars.env lacks %s:\n%s", want, content)
				}
			}
		})
	}
}

func TestInstallLocationPrompt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	logger, _ := core.NewLogger("")
	noPick := func(*core.Logger) (string, error) { t.Fatal("picker opened"); return "", nil }
	for answer, want := range map[string]string{
		"":                filepath.Join(home, "Games", "Bellum"),
		"~/Games":         filepath.Join(home, "Games", "Bellum"),
		"/mnt/ssd":        "/mnt/ssd/Bellum",
		"/mnt/ssd/Bellum": "/mnt/ssd/Bellum",
	} {
		got, err := promptInstallLocationWith(logger, scripted(answer), noPick)
		if err != nil || got != want {
			t.Errorf("answer %q: got %q, %v; want %q", answer, got, err, want)
		}
	}
	picked := false
	pick := func(*core.Logger) (string, error) { picked = true; return "/picked/Bellum", nil }
	if got, err := promptInstallLocationWith(logger, scripted("b"), pick); err != nil || !picked || got != "/picked/Bellum" {
		t.Fatalf("browse: %q, %v, picked=%t", got, err, picked)
	}
}
