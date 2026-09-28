// Command screenshots replays a real install (RX 9070 XT, CachyOS, KDE
// Wayland, v2.2.0 release candidate) through the installer's own terminal UI
// code, so the README screenshots can be regenerated whenever the UI changes.
// It runs no install steps: the output lines come from that session and the
// timings are shortened. Run it through tools/screenshots/render.py.
//
//	go run ./tools/screenshots <scene>   # scene: start, review, install, finish
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"bellum-installer/pkg/config"
	"bellum-installer/pkg/core"
)

const prefix = "/home/player/Games/Bellum"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: screenshots start|review|install|finish")
		os.Exit(2)
	}
	core.AssumeYes = true
	logger, err := core.NewLogger("")
	if err != nil {
		panic(err)
	}
	switch os.Args[1] {
	case "start":
		start(logger)
	case "review":
		review()
	case "install":
		install(logger)
	case "finish":
		finish(logger)
	default:
		fmt.Fprintln(os.Stderr, "unknown scene", os.Args[1])
		os.Exit(2)
	}
}

// banner mirrors printInstallerBanner in bellum-installer/main.go.
func banner() {
	proton := strings.TrimSuffix(strings.TrimPrefix(config.DefaultVersions.ProtonVer, "proton-cachyos-"), "-x86_64")
	winetricks := strings.TrimSuffix(strings.TrimPrefix(config.DefaultVersions.WinetricksVer, "bundled with pinned Proton ("), ")")
	dim := func(s string) string { return core.ColorGrayBold + s + core.ColorReset }
	core.Banner(core.LogoLines("packages/launcher_1_256x256x32.png", 26), []string{
		core.ColorBold + "B E L L U M" + core.ColorReset,
		core.ColorBoldCyan + "Linux Installer" + core.ColorReset + " " + dim(config.InstallerVersion),
		"",
		dim("Proton-CachyOS ") + proton,
		dim("umu-launcher   ") + config.DefaultVersions.UMUVersion,
		dim("winetricks     ") + winetricks,
		"",
		dim("Community project, not affiliated with Astarte Industries"),
		dim("github.com/Ch3w3y/bellum-linux-installer"),
	})
}

func start(logger *core.Logger) {
	banner()
	core.Step(1, 5, "Install location")
	fmt.Println("  Where should Bellum be installed? Use a fast SSD with plenty of free space.")
	fmt.Printf("  %sEnter%s for %s%s%s, type another folder, or %sb%s to browse.\n", core.Bold, core.ColorReset, core.ColorBoldYellow, prefix, core.ColorReset, core.Bold, core.ColorReset)
	fmt.Printf("  %s›%s \n", core.ColorBoldCyan, core.ColorReset)
	core.Step(2, 5, "Checking your system")
	logger.Info("[OK] GPU: AMD RDNA4  (AMD Radeon RX 9070 XT (radeonsi, gfx1201, ACO))")
	logger.Info("Detected: CachyOS Linux · RDNA4 · Wayland (verified)")
	logger.Info("Install folder: " + core.Colorize(prefix, core.ColorBoldYellow))
	for _, ok := range []string{
		"Install folder is writable",
		"Install folder is on an SSD/NVMe drive",
		"python3 found",
		"flock found",
		"Proton EasyAntiCheat Runtime found in your Steam library",
		"Your system is ready",
	} {
		logger.Info("[OK] " + ok)
	}
	logger.Info("Display session: Wayland KDE (the game runs through XWayland)")
}

func review() {
	core.Step(3, 5, "Review")
	core.PrintInstallerSummary(config.DefaultVersions.ProtonVer, config.DefaultVersions.WinetricksVer, "", "integrated with pinned Proton",
		prefix, "", "CachyOS Linux · RDNA4 · Wayland (verified)", "AMD", "AMD RDNA4: native FSR4 (FP8) through Proton", "")
	core.ConfirmProceed()
}

func task(label string, d time.Duration) {
	t := core.StartTask(label)
	time.Sleep(d)
	t.Done(nil)
}

func download(label string, size int64, d time.Duration) {
	p := core.NewProgress(label, size)
	const steps = 40
	for i := 0; i < steps; i++ {
		time.Sleep(d / steps)
		p.Add(size / steps)
	}
	p.Add(size - p.Done())
	p.Finish(nil)
}

func install(logger *core.Logger) {
	core.Step(4, 5, "Downloading the runtime")
	download("Proton-CachyOS 11.0-20260703-slr", 320<<20, 2*time.Second)
	task("Unpacking Proton", 1500*time.Millisecond)
	core.Step(5, 5, "Installing Bellum")
	download("Astarte Launcher installer", 53<<20, 600*time.Millisecond)
	task("Creating the Wine prefix with Proton", 1100*time.Millisecond)
	for _, verb := range []string{"Visual C++ 2015-2022 runtime", "D3D compiler 47", "FAudio", ".NET 9 desktop runtime", "MFC 14 runtime"} {
		task("Installing "+verb, 1100*time.Millisecond)
	}
	fmt.Println()
	logger.Info("The Astarte Launcher installer opens next. Follow its prompts; this window waits for it.")
	task("Running the Astarte Launcher installer", 1100*time.Millisecond)
	logger.Info("[OK] Astarte Launcher updated to v1.4.2")
	task("Setting Windows 11 mode", 1100*time.Millisecond)
	task("Tuning window and input settings", 1100*time.Millisecond)
	task("Setting DLL overrides", 1100*time.Millisecond)
}

// finish mirrors printFinish in bellum-installer/main.go.
func finish(logger *core.Logger) {
	logger.Info("[OK] Game launcher installed in ~/.local/bin/Bellum")
	logger.Info("[OK] Configuration phase complete!")
	fmt.Println()
	fmt.Printf("  %s✔ %s%s\n\n", core.ColorBoldGreen, "Bellum is installed", core.ColorReset)
	fmt.Println("  Start it from:")
	fmt.Printf("    %s▸%s the %sBellum%s shortcut on your desktop\n", core.ColorBoldCyan, core.ColorReset, core.Bold, core.ColorReset)
	fmt.Printf("    %s▸%s your app menu, under Games\n", core.ColorBoldCyan, core.ColorReset)
	fmt.Printf("    %s▸%s the %sBellum%s command in a terminal\n", core.ColorBoldCyan, core.ColorReset, core.Bold, core.ColorReset)
	fmt.Println()
	fmt.Printf("  %sKeep the Astarte Launcher open while you play. Settings: %s/launch_vars.env%s\n", core.ColorGrayBold, prefix, core.ColorReset)
	fmt.Printf("  %sTo update later, run the same install command again.%s\n\n", core.ColorGrayBold, core.ColorReset)
}
