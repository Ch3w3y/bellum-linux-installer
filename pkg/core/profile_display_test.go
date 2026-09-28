package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The review-screen label, launch profile and Game Mode steering for every
// golden platform case.
func TestProfileDisplayFromGoldenCases(t *testing.T) {
	for name, want := range map[string]struct {
		label    string
		launch   LaunchProfile
		gameMode bool
	}{
		"deck-lcd-gamemode":                 {"Steam Deck LCD · SteamOS 3 · RDNA2 · Game Mode", LaunchAMDBaseline, true},
		"deck-oled-desktop":                 {"Steam Deck OLED · SteamOS 3 · RDNA2 · Wayland", LaunchAMDBaseline, true},
		"desktop-arch-rdna4":                {"CachyOS Linux · RDNA4 · Wayland", LaunchAMDRDNA4, false},
		"steam-machine-synthetic":           {"Steam Machine (provisional) · SteamOS 3 · RDNA3", LaunchAMDRDNA3, true},
		"nonvalve-gfx11xx-negative":         {"RDNA3", LaunchAMDRDNA3, false},
		"desktop-fedora-nvidia-proprietary": {"NVIDIA Ada · X11", LaunchNVIDIARTX, false},
	} {
		t.Run(name, func(t *testing.T) {
			sources, _ := loadGolden(t, filepath.Join("testdata", "platform", name))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			p, err := DetectPlatformWith(ctx, sources)
			if err != nil {
				t.Fatal(err)
			}
			got := PlatformLabel(p)
			if !strings.Contains(got, want.label) {
				t.Errorf("label %q, want it to contain %q", got, want.label)
			}
			if lp := LaunchProfileFor(p.GPU); lp != want.launch {
				t.Errorf("launch profile %s, want %s", lp, want.launch)
			}
			if SupportsGameModeIntegration(p) != want.gameMode {
				t.Errorf("game mode integration = %t, want %t", !want.gameMode, want.gameMode)
			}
		})
	}
}

func TestLaunchProfilePolicy(t *testing.T) {
	for _, tc := range []struct {
		caps          GPUCapabilities
		want          LaunchProfile
		fsr4, rdna3wa bool
		status        ProfileStatus
	}{
		{GPUCapabilities{Vendor: GPUAMD, Generation: "RDNA4", FSR41: true}, LaunchAMDRDNA4, true, false, ProfileVerified},
		{GPUCapabilities{Vendor: GPUAMD, Generation: "RDNA4"}, LaunchAMDBaseline, false, false, ProfileExpected},
		{GPUCapabilities{Vendor: GPUAMD, Generation: "RDNA3"}, LaunchAMDRDNA3, false, true, ProfileExpected},
		{GPUCapabilities{Vendor: GPUAMD, Generation: "RDNA2"}, LaunchAMDBaseline, false, false, ProfileExpected},
		{GPUCapabilities{Vendor: GPUAMD, Generation: "RDNA1"}, LaunchAMDBaseline, false, false, ProfileExpected},
		{GPUCapabilities{Vendor: GPUAMD, Generation: "GCN/CDNA"}, LaunchAMDBaseline, false, false, ProfileExpected},
		{GPUCapabilities{Vendor: GPUAMD}, LaunchAMDBaseline, false, false, ProfileExpected},
		// Ambiguity never unlocks a generation-specific path.
		{GPUCapabilities{Vendor: GPUAMD, Generation: "RDNA3", Ambiguous: true}, LaunchAMDBaseline, false, false, ProfileExpected},
		{GPUCapabilities{Vendor: GPUNVIDIA, NVAPI: true}, LaunchNVIDIARTX, false, false, ProfileExpected},
		{GPUCapabilities{Vendor: GPUNVIDIA}, LaunchNVIDIABasic, false, false, ProfileExpected},
		{GPUCapabilities{Vendor: GPUIntel}, LaunchIntel, false, false, ProfileExpected},
		{GPUCapabilities{Vendor: GPUUnknown}, LaunchGeneric, false, false, ProfileExpected},
		{GPUCapabilities{}, LaunchGeneric, false, false, ProfileExpected},
	} {
		got := LaunchProfileFor(tc.caps)
		if got != tc.want || got.UsesFSR4Upgrade() != tc.fsr4 || got.UsesRDNA3Workaround() != tc.rdna3wa || got.Status() != tc.status {
			t.Errorf("%+v: got %s (fsr4=%t rdna3wa=%t %s)", tc.caps, got, got.UsesFSR4Upgrade(), got.UsesRDNA3Workaround(), got.Status())
		}
	}
}

func TestPlatformLabelParts(t *testing.T) {
	p := Platform{
		OS:       OSInfo{ID: "steamos", VersionID: "3.7.13"},
		Hardware: HardwareInfo{SKU: SKUDeckOLED},
		GPU:      GPUCapabilities{Vendor: GPUAMD, Generation: "RDNA2"},
		Session:  SessionInfo{Kind: SessionGamescope, GameMode: TriYes},
	}
	if got, want := PlatformLabel(p), "Steam Deck OLED · SteamOS 3 · RDNA2 · Game Mode"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// Untrusted os-release text is sanitized before display.
	p = Platform{OS: OSInfo{ID: "x", PrettyName: "Evil\x1b[31m OS\n"}, GPU: GPUCapabilities{Vendor: GPUUnknown}}
	if got, want := PlatformLabel(p), "Evil OS · Unknown GPU"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := OSLabel(OSInfo{ID: "bazzite", VariantID: "bazzite-deck"}); got != "Bazzite (Deck image)" {
		t.Fatalf("bazzite deck: %q", got)
	}
	if !SupportsGameModeIntegration(Platform{OS: OSInfo{ID: "bazzite", VariantID: "bazzite-deck"}}) {
		t.Fatal("Bazzite Deck images should steer towards Steam")
	}
	if SupportsGameModeIntegration(Platform{OS: OSInfo{ID: "bazzite"}, Session: SessionInfo{Kind: SessionWayland, GameMode: TriNo}}) {
		t.Fatal("Bazzite desktop images are ordinary desktops")
	}
	if got := GPULabel(GPUCapabilities{Vendor: GPUNVIDIA, Generation: "Ada"}); got != "NVIDIA Ada" {
		t.Fatalf("nvidia label %q", got)
	}
}
