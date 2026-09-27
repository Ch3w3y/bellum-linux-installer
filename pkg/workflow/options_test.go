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

func TestSingleConfigPerVendor(t *testing.T) {
	logger, _ := core.NewLogger("")
	rdna3 := core.GPUCapabilities{Vendor: core.GPUAMD, Generation: "RDNA3"}
	for _, tc := range []struct {
		name         string
		caps         core.GPUCapabilities
		want, absent []string
	}{
		// DLSS works through Proton's default NVAPI + nvngx copy in both presets.
		{"rtx", rtx, []string{`PROTON_NVIDIA_LIBS="1"`, `DXVK_ENABLE_NVAPI="1"`}, []string{"PROTON_ENABLE_NVAPI", "PROTON_ENABLE_NGX_UPDATER", "PROTON_VKD3D_HEAP"}},
		// RDNA4 gets the forced FSR4 offer and never the FP16 emulation switch.
		{"rdna4", rdna4, []string{`PROTON_FSR4_UPGRADE="1"`}, []string{"wmma_rdna3_workaround", "PROTON_FSR4_RDNA3_UPGRADE"}},
		{"rdna3", rdna3, []string{`PROTON_FSR4_UPGRADE="0"`, "wmma_rdna3_workaround"}, []string{"PROTON_FSR4_RDNA3_UPGRADE"}},
		{"intel", core.GPUCapabilities{Vendor: core.GPUIntel}, []string{"PROTON_DLSS_UPGRADE=0"}, []string{"PROTON_FSR4_RDNA3_UPGRADE"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fakeFileStore{written: map[string][]byte{}}
			boundaries := WorkflowBoundaries{Commands: fakeCommands{}, Files: files, MutatePrefix: func(core.RunMode, []string, *core.Logger, string) error { return nil }}
			config := ConfigureConfig{WINEPREFIX: "/prefix", ProtonPath: "/proton", GPUCapabilities: tc.caps, IsFSR41: tc.caps.FSR41}
			if err := RunConfigurationWithBoundaries(config, logger, boundaries); err != nil {
				t.Fatal(err)
			}
			content := string(files.written["/prefix/launch_vars.env"])
			for _, want := range append(tc.want, `BELLUM_GAMEMODE="0"`, "BELLUM_UMU_RUN=") {
				if !strings.Contains(content, want) {
					t.Errorf("launch_vars.env lacks %s:\n%s", want, content)
				}
			}
			for _, bad := range append(tc.absent, "LOWLATENCY") {
				if strings.Contains(content, bad) {
					t.Errorf("launch_vars.env should not contain %s:\n%s", bad, content)
				}
			}
		})
	}
}

func TestConfigSummaryPerVendor(t *testing.T) {
	for caps, want := range map[core.GPUCapabilities]string{
		rtx:   "DLSS",
		rdna4: "RDNA4",
		{Vendor: core.GPUAMD, Generation: "RDNA3"}: "FSR4 through Proton where",
		{Vendor: core.GPUIntel}:                    "Intel",
		{Vendor: core.GPUUnknown}:                  "Unrecognised",
	} {
		if got := ConfigSummary(caps); !strings.Contains(got, want) {
			t.Errorf("%+v: %q lacks %q", caps, got, want)
		}
	}
}

func TestDisplaySession(t *testing.T) {
	for want, env := range map[string]map[string]string{
		"gamescope":    {"XDG_CURRENT_DESKTOP": "gamescope", "WAYLAND_DISPLAY": "gamescope-0"},
		"Wayland KDE":  {"XDG_CURRENT_DESKTOP": "KDE", "WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"},
		"X11 GNOME":    {"XDG_CURRENT_DESKTOP": "GNOME", "DISPLAY": ":0"},
		"no graphical": {},
	} {
		got := displaySession(func(k string) string { return env[k] })
		if !strings.HasPrefix(got, want) {
			t.Errorf("env %v: got %q, want prefix %q", env, got, want)
		}
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
