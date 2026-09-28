package launchers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAllVendorsUseUMUAndEAC(t *testing.T) {
	for _, vendor := range []string{"AMD", "NVIDIA", "Intel"} {
		script := generateWrapperContent(LauncherConfig{Wineprefix: "/tmp/Bellum's prefix", Protonpath: "/proton", GPUType: vendor})
		if !strings.Contains(script, `cmd=("$UMU_RUN" "$LAUNCHER_EXE" "$@")`) || !strings.Contains(script, `exec "${cmd[@]}"`) || !strings.Contains(script, "PROTON_EAC_RUNTIME") {
			t.Fatalf("%s wrapper does not require umu and EAC", vendor)
		}
		if strings.Contains(script, "wineboot") || strings.Contains(script, "\nwine ") || strings.Contains(script, "cd \"$GAME_DIR\"") {
			t.Fatalf("%s wrapper uses legacy Wine/game path", vendor)
		}
		if !strings.Contains(script, `LAUNCH_VARS='/tmp/Bellum'"'"'s prefix/launch_vars.env'`) {
			t.Fatalf("%s wrapper fails to quote paths", vendor)
		}
		for _, want := range []string{"flock -n 9", "chmod 0700 \"$WINEPREFIX\"", "chmod 0600 \"$LAUNCH_VARS\"", "chmod 0600 \"$WINEPREFIX/launcher.log\""} {
			if !strings.Contains(script, want) {
				t.Fatalf("%s wrapper missing %q", vendor, want)
			}
		}
	}
}

func TestWrapperSecondClickReturnsWhileSessionRuns(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "Bellum prefix")
	launcher := filepath.Join(prefix, "drive_c/users/steamuser/AppData/Local/Astarte Industries/Astarte Launcher/AstarteLauncher.exe")
	proton := filepath.Join(t.TempDir(), "proton build")
	runtimeDir := filepath.Join(t.TempDir(), "EAC runtime")
	bin := t.TempDir()
	for _, dir := range []string{filepath.Dir(launcher), proton, runtimeDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{launcher, filepath.Join(proton, "proton")} {
		if err := os.WriteFile(path, []byte("stub"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	vars := "export WINEPREFIX='" + prefix + "'\nexport PROTONPATH='" + proton + "'\nexport PROTON_EAC_RUNTIME='" + runtimeDir + "'\n"
	if err := os.WriteFile(filepath.Join(prefix, "launch_vars.env"), []byte(vars), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "umu-run"), []byte("#!/bin/sh\nsleep 2\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("BASH_ENV", "")
	if _, err := exec.LookPath("umu-run"); err != nil {
		t.Fatalf("fake umu-run missing from PATH: %v", err)
	}
	wrapper := filepath.Join(t.TempDir(), "Bellum")
	if err := os.WriteFile(wrapper, []byte(generateWrapperContent(LauncherConfig{Wineprefix: prefix})), 0700); err != nil {
		t.Fatal(err)
	}
	first := exec.Command("bash", wrapper)
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Process.Kill(); _ = first.Wait() })
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(prefix, "launcher.log")); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	second := exec.Command("bash", wrapper)
	start := time.Now()
	output, err := second.CombinedOutput()
	if err != nil || time.Since(start) > time.Second || !strings.Contains(string(output), "already running") {
		t.Fatalf("second click blocked or failed: %v, %s", err, output)
	}
	for path, want := range map[string]os.FileMode{prefix: 0700, filepath.Join(prefix, "launch_vars.env"): 0600, filepath.Join(prefix, "launcher.log"): 0600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("%s mode: %v, %v; want %v", path, info, err, want)
		}
	}
}

func TestUnknownGPUStillGetsLauncherAndDesktopEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	prefix := filepath.Join(t.TempDir(), "Bellum")
	if err := GenerateLauncher(LauncherConfig{Wineprefix: prefix, Protonpath: "/proton", GPUType: "Unknown"}); err != nil {
		t.Fatalf("unknown GPU failed launcher generation: %v", err)
	}
	for _, path := range []string{
		filepath.Join(home, ".local", "bin", "Bellum"),
		filepath.Join(home, ".local", "share", "applications", "Bellum.desktop"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
}

func TestWrapperChainsGameModeAndGamescope(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "Bellum")
	launcher := filepath.Join(prefix, "drive_c/users/steamuser/AppData/Local/Astarte Industries/Astarte Launcher/AstarteLauncher.exe")
	proton := t.TempDir()
	runtimeDir := t.TempDir()
	bin := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(launcher), 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		launcher:                          "stub",
		filepath.Join(proton, "proton"):   "stub",
		filepath.Join(bin, "umu-run"):     "#!/bin/sh\n",
		filepath.Join(bin, "gamemoderun"): "#!/bin/sh\n",
		filepath.Join(bin, "gamescope"):   "#!/bin/sh\necho \"gamescope $*\"\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
	}
	vars := "export WINEPREFIX='" + prefix + "'\nexport PROTONPATH='" + proton + "'\nexport PROTON_EAC_RUNTIME='" + runtimeDir + "'\n" +
		"export BELLUM_GAMEMODE=1\nexport BELLUM_GAMESCOPE=1\n"
	if err := os.WriteFile(filepath.Join(prefix, "launch_vars.env"), []byte(vars), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "") // not inside Game Mode
	t.Setenv("BASH_ENV", "")
	wrapper := filepath.Join(t.TempDir(), "Bellum")
	if err := os.WriteFile(wrapper, []byte(generateWrapperContent(LauncherConfig{Wineprefix: prefix})), 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("bash", wrapper, "--arg").CombinedOutput(); err != nil {
		t.Fatalf("wrapper failed: %v %s", err, out)
	}
	log, err := os.ReadFile(filepath.Join(prefix, "launcher.log"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "gamescope -- gamemoderun " + filepath.Join(bin, "umu-run") + " " + launcher + " --arg"; !strings.Contains(string(log), want) {
		t.Fatalf("launch chain = %q, want %q", log, want)
	}
}

func TestWrapperUpdatesLauncherFirstAndStartsEvenIfUpdateFails(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "Bellum prefix")
	launcher := filepath.Join(prefix, "drive_c/users/steamuser/AppData/Local/Astarte Industries/Astarte Launcher/AstarteLauncher.exe")
	proton := filepath.Join(t.TempDir(), "proton build")
	runtimeDir := filepath.Join(t.TempDir(), "EAC runtime")
	bin := t.TempDir()
	for _, dir := range []string{filepath.Dir(launcher), proton, runtimeDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{launcher, filepath.Join(proton, "proton")} {
		if err := os.WriteFile(path, []byte("stub"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	trace := filepath.Join(t.TempDir(), "trace")
	vars := "export WINEPREFIX='" + prefix + "'\nexport PROTONPATH='" + proton + "'\nexport PROTON_EAC_RUNTIME='" + runtimeDir + "'\n"
	if err := os.WriteFile(filepath.Join(prefix, "launch_vars.env"), []byte(vars), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "umu-run"), []byte("#!/bin/sh\necho umu-run >> '"+trace+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(t.TempDir(), "bellum tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho \"tool $1 $2\" >> '"+trace+"'\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("BASH_ENV", "")

	for _, cfg := range []LauncherConfig{
		{Wineprefix: prefix, ToolPath: tool},
		{Wineprefix: prefix, ToolPath: filepath.Join(t.TempDir(), "missing")},
	} {
		_ = os.Remove(trace)
		wrapper := filepath.Join(t.TempDir(), "Bellum")
		if err := os.WriteFile(wrapper, []byte(generateWrapperContent(cfg)), 0700); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("bash", wrapper).CombinedOutput(); err != nil {
			t.Fatalf("wrapper failed: %v %s", err, out)
		}
		got, _ := os.ReadFile(trace)
		want := "umu-run\n"
		if cfg.ToolPath == tool {
			want = "tool update-launcher " + prefix + "\numu-run\n"
		}
		if string(got) != want {
			t.Fatalf("trace = %q, want %q", got, want)
		}
	}
	if !strings.Contains(generateWrapperContent(LauncherConfig{Wineprefix: prefix, ToolPath: tool}), "update-launcher") ||
		strings.Contains(generateWrapperContent(LauncherConfig{Wineprefix: prefix}), "update-launcher") {
		t.Fatal("update block not tied to ToolPath")
	}
}

// wrapperFixture builds a runnable wrapper with stub Proton, umu-run and
// gamescope, and returns it with the prefix and the stub bin directory.
func wrapperFixture(t *testing.T, extraVars string, stubs map[string]string) (wrapper, prefix, bin string) {
	t.Helper()
	prefix = filepath.Join(t.TempDir(), "Bellum")
	launcher := filepath.Join(prefix, "drive_c/users/steamuser/AppData/Local/Astarte Industries/Astarte Launcher/AstarteLauncher.exe")
	proton := t.TempDir()
	runtimeDir := t.TempDir()
	bin = t.TempDir()
	if err := os.MkdirAll(filepath.Dir(launcher), 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		launcher:                        "stub",
		filepath.Join(proton, "proton"): "stub",
		filepath.Join(bin, "umu-run"):   "#!/bin/sh\necho \"umu-run $*\"\n",
	}
	for name, content := range stubs {
		files[filepath.Join(bin, name)] = content
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
	}
	vars := "export WINEPREFIX='" + prefix + "'\nexport PROTONPATH='" + proton + "'\nexport PROTON_EAC_RUNTIME='" + runtimeDir + "'\n" + extraVars
	if err := os.WriteFile(filepath.Join(prefix, "launch_vars.env"), []byte(vars), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("BASH_ENV", "")
	wrapper = filepath.Join(t.TempDir(), "Bellum")
	if err := os.WriteFile(wrapper, []byte(generateWrapperContent(LauncherConfig{Wineprefix: prefix})), 0700); err != nil {
		t.Fatal(err)
	}
	return wrapper, prefix, bin
}

func TestWrapperNeverNestsGamescopeInGameMode(t *testing.T) {
	wrapper, prefix, _ := wrapperFixture(t, "export BELLUM_GAMESCOPE=1\n", map[string]string{"gamescope": "#!/bin/sh\necho \"gamescope $*\"\n"})
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "gamescope-0")
	if out, err := exec.Command("bash", wrapper).CombinedOutput(); err != nil {
		t.Fatalf("wrapper failed: %v %s", err, out)
	}
	log, _ := os.ReadFile(filepath.Join(prefix, "launcher.log"))
	if strings.Contains(string(log), "gamescope --") || !strings.Contains(string(log), "already running inside gamescope") || !strings.Contains(string(log), "umu-run") {
		t.Fatalf("launcher.log:\n%s", log)
	}
}

func TestWrapperHintsAtDoubleInputOutsideSteam(t *testing.T) {
	// pgrep reports a running Steam.
	wrapper, prefix, _ := wrapperFixture(t, "", map[string]string{"pgrep": "#!/bin/sh\nexit 0\n"})
	t.Setenv("SteamGameId", "")
	if out, err := exec.Command("bash", wrapper).CombinedOutput(); err != nil {
		t.Fatalf("wrapper failed: %v %s", err, out)
	}
	log, _ := os.ReadFile(filepath.Join(prefix, "launcher.log"))
	if !strings.Contains(string(log), "double input") {
		t.Fatalf("no double-input hint:\n%s", log)
	}

	// Started from Steam: no hint.
	wrapper, prefix, _ = wrapperFixture(t, "", map[string]string{"pgrep": "#!/bin/sh\nexit 0\n"})
	t.Setenv("SteamGameId", "12345")
	if out, err := exec.Command("bash", wrapper).CombinedOutput(); err != nil {
		t.Fatalf("wrapper failed: %v %s", err, out)
	}
	log, _ = os.ReadFile(filepath.Join(prefix, "launcher.log"))
	if strings.Contains(string(log), "double input") {
		t.Fatalf("hint when started from Steam:\n%s", log)
	}
}
