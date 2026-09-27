package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bellum-installer/pkg/core"
)

// Preset selects how much of Proton's vendor-specific upscaler support the
// launch settings turn on. Both presets use only Proton's own flags; the
// installer never adds or replaces DLLs (#11).
type Preset string

const (
	PresetStable      Preset = "stable"
	PresetPerformance Preset = "performance"
)

// InstallOptions are the guided choices written into launch_vars.env.
type InstallOptions struct {
	Preset    Preset
	MangoHud  bool
	Gamescope bool
	GameMode  bool
}

// DefaultInstallOptions is what pressing Enter through every question gives.
func DefaultInstallOptions() InstallOptions {
	return InstallOptions{Preset: PresetStable}
}

// PerformanceFeatures lists, in plain language, what the Performance preset
// turns on for this GPU. An empty list means the GPU has no vendor extras and
// only the Stable preset is offered.
func PerformanceFeatures(caps core.GPUCapabilities) []string {
	var features []string
	if caps.Vendor == core.GPUNVIDIA && caps.NVAPI {
		features = append(features, "NVAPI and the NVIDIA driver libraries, so the game can offer DLSS and Reflex (PROTON_ENABLE_NVAPI=1, PROTON_NVIDIA_LIBS=1)")
	}
	if caps.Vendor == core.GPUAMD && caps.Generation == "RDNA4" && !caps.Ambiguous && caps.FSR41 {
		features = append(features, "Proton's FSR4 upgrade for RDNA4 (PROTON_FSR4_UPGRADE=1)")
	}
	return features
}

// optionalExtra is a tool the wrapper can wrap the game with.
type optionalExtra struct {
	binary, label, envVar, pkg string
	enable                     func(*InstallOptions)
}

var optionalExtras = []optionalExtra{
	{"mangohud", "MangoHud performance overlay (FPS, frame times)", "BELLUM_MANGOHUD", "mangohud", func(o *InstallOptions) { o.MangoHud = true }},
	{"gamemoderun", "Feral GameMode (asks the system for performance mode while playing)", "BELLUM_GAMEMODE", "gamemode", func(o *InstallOptions) { o.GameMode = true }},
	{"gamescope", "gamescope compositor (runs the game in its own session; advanced)", "BELLUM_GAMESCOPE", "gamescope", func(o *InstallOptions) { o.Gamescope = true }},
}

// ChooseInstallOptions asks for the preset and optional extras. Every
// question has a default, so pressing Enter throughout gives Stable with no
// extras.
func ChooseInstallOptions(caps core.GPUCapabilities, logger *core.Logger) InstallOptions {
	return chooseInstallOptionsWith(caps, logger, core.Prompt, DefaultBoundaries.Commands)
}

func chooseInstallOptionsWith(caps core.GPUCapabilities, logger *core.Logger, prompt func(string, string) string, commands CommandRunner) InstallOptions {
	opts := DefaultInstallOptions()
	fmt.Println()
	features := PerformanceFeatures(caps)
	if len(features) == 0 {
		logger.Info("Preset: Stable. Your GPU has no vendor-specific extras to enable.")
	} else {
		fmt.Println("Choose a preset:")
		fmt.Println("  1) Stable (recommended): Proton defaults that work on every setup.")
		fmt.Println("  2) Performance: also enables")
		for _, f := range features {
			fmt.Println("       - " + f)
		}
		for {
			answer := strings.TrimSpace(prompt("Preset [1]: ", "1"))
			switch strings.ToLower(answer) {
			case "1", "s", "stable":
				opts.Preset = PresetStable
			case "2", "p", "performance":
				opts.Preset = PresetPerformance
			default:
				fmt.Println("Please type 1 or 2.")
				continue
			}
			break
		}
	}

	fmt.Println()
	for _, extra := range optionalExtras {
		if DiscoverExecutable(extra.binary, commands) == "" {
			logger.Info(fmt.Sprintf("Optional: install the %s package for the %s. You can turn it on later with %s=1 in launch_vars.env.", extra.pkg, extra.label, extra.envVar))
			continue
		}
		answer := strings.ToLower(strings.TrimSpace(prompt(fmt.Sprintf("Enable the %s? (y/N): ", extra.label), "n")))
		if answer == "y" || answer == "yes" {
			extra.enable(&opts)
		}
	}
	return opts
}

// Summary describes the choices for the confirmation screen.
func (o InstallOptions) Summary() string {
	preset := "Stable"
	if o.Preset == PresetPerformance {
		preset = "Performance"
	}
	var extras []string
	if o.MangoHud {
		extras = append(extras, "MangoHud")
	}
	if o.GameMode {
		extras = append(extras, "GameMode")
	}
	if o.Gamescope {
		extras = append(extras, "gamescope")
	}
	if len(extras) == 0 {
		return preset + ", no extras"
	}
	return preset + " + " + strings.Join(extras, ", ")
}

// launchVarsExtras returns the BELLUM_* toggles the wrapper reads.
func (o InstallOptions) launchVarsExtras() string {
	flag := func(on bool) string {
		if on {
			return "1"
		}
		return "0"
	}
	return "# Optional extras (set to 1 to enable)\n" +
		"export BELLUM_MANGOHUD=\"" + flag(o.MangoHud) + "\"\n" +
		"export BELLUM_GAMEMODE=\"" + flag(o.GameMode) + "\"\n" +
		"export BELLUM_GAMESCOPE=\"" + flag(o.Gamescope) + "\"\n"
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
	fmt.Println()
	fmt.Println("Where should Bellum be installed? Use a fast SSD with plenty of free space.")
	answer := strings.TrimSpace(prompt(fmt.Sprintf("Press Enter for %s, type a folder, or type b to browse: ", def), def))
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
