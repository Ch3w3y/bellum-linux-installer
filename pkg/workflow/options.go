package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bellum-installer/pkg/core"
)

// There is one configuration: the most stable one, then the fastest that
// stays stable. It differs only by the detected launch profile (see
// core.LaunchProfileFor and configure.go); nothing is left for the user to
// choose. Every setting is a Proton or driver feature; the installer never
// adds or replaces DLLs (#11).

// ConfigSummary describes, for the confirmation screen, what the launch
// settings will be on this GPU.
func ConfigSummary(caps core.GPUCapabilities) string {
	switch core.LaunchProfileFor(caps) {
	case core.LaunchNVIDIARTX:
		if !caps.DLSS {
			// GTX 16-series: Turing without the tensor cores DLSS needs.
			return "NVIDIA: Reflex through the driver (NVAPI), CUDA/NVENC bridges"
		}
		return "NVIDIA: DLSS and Reflex through the driver (NVAPI), CUDA/NVENC bridges"
	case core.LaunchNVIDIABasic:
		return "NVIDIA (no RTX features): standard Proton settings"
	case core.LaunchAMDRDNA4:
		return "AMD RDNA4: native FSR4 (FP8) through Proton"
	case core.LaunchAMDRDNA3:
		return "AMD RDNA3: FSR4 through Proton where your GPU supports it"
	case core.LaunchAMDBaseline:
		return "AMD: the game's own FSR 3.x (no FSR4 emulation)"
	case core.LaunchIntel:
		return "Intel: standard Proton settings"
	}
	return "Unrecognised GPU: standard Proton settings"
}

// ProfileSummary is the review-screen line for the detected platform and the
// evidence behind its launch profile, for example
// "Steam Deck OLED · SteamOS 3 · RDNA2 · Game Mode (expected)".
func ProfileSummary(p core.Platform) string {
	return core.PlatformLabel(p) + " (" + string(core.LaunchProfileFor(p.GPU).Status()) + ")"
}

// DisplaySession names the graphical session. Bellum runs through XWayland
// on Wayland sessions (the pinned Proton only uses its Wayland driver when
// PROTON_ENABLE_WAYLAND is set, which stays off because it's experimental),
// so the launch settings are the same on X11, Wayland and gamescope.
func DisplaySession() string {
	return displaySession(os.Getenv)
}

func displaySession(env func(string) string) string {
	// Session precedence is owned by core; this wrapper keeps the existing
	// presentation string for current callers until step 3 migrates the
	// display. The new UI uses typed session information, never this string.
	info := core.ClassifySessionFunc(env)
	switch info.Kind {
	case core.SessionGamescope:
		return "gamescope (Steam Deck / Game Mode)"
	case core.SessionWayland:
		return strings.TrimSpace("Wayland " + info.Desktop + " (the game runs through XWayland)")
	case core.SessionX11:
		return strings.TrimSpace("X11 " + info.Desktop)
	}
	return "no graphical session detected"
}

// launchVarsToggles documents the optional wrapper extras. They stay off:
// each one adds a layer that hasn't been validated with Easy Anti-Cheat.
func launchVarsToggles() string {
	return "# Optional extras, off by default (set to 1 to try them)\n" +
		"export BELLUM_MANGOHUD=\"0\"   # MangoHud overlay\n" +
		"export BELLUM_GAMEMODE=\"0\"   # Feral GameMode\n" +
		"export BELLUM_GAMESCOPE=\"0\"  # run inside gamescope\n"
}

// DefaultInstallLocation is where Bellum goes when the user just presses Enter.
func DefaultInstallLocation() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Games", "Bellum")
}

// PromptInstallLocation asks where to install, defaulting to ~/Games/Bellum.
// Typing "b" opens the graphical folder picker.
func PromptInstallLocation(logger *core.Logger) (string, error) {
	return promptInstallLocationWith(logger, core.Prompt, PickWINEPREFIXWithGUI)
}

func promptInstallLocationWith(logger *core.Logger, prompt func(string, string) string, pick func(*core.Logger) (string, error)) (string, error) {
	def := DefaultInstallLocation()
	fmt.Println("  Where should Bellum be installed? Use a fast SSD with plenty of free space.")
	fmt.Printf("  %sEnter%s for %s%s%s, type another folder, or %sb%s to browse.\n", core.Bold, core.ColorReset, core.ColorBoldYellow, def, core.ColorReset, core.Bold, core.ColorReset)
	answer := strings.TrimSpace(prompt("  "+core.ColorBoldCyan+"›"+core.ColorReset+" ", def))
	switch {
	case strings.EqualFold(answer, "b"):
		return pick(logger)
	case answer == "~":
		answer, _ = os.UserHomeDir()
	case strings.HasPrefix(answer, "~/"):
		home, _ := os.UserHomeDir()
		answer = filepath.Join(home, answer[2:])
	}
	return ResolvePrefixPath(answer)
}
