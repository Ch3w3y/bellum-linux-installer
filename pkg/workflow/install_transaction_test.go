package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bellum-installer/pkg/core"
	"bellum-installer/pkg/launchers"
)

type transactionCommands struct{}

func (transactionCommands) Run(core.RunMode, []string, *core.Logger, string) error { return nil }
func (transactionCommands) Output([]string) (string, error)                        { return "", nil }
func (transactionCommands) LookPath(string) string                                 { return "" }

func TestInstallFailureRollsBackAndRetrySucceeds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	parent := t.TempDir()
	prefix := filepath.Join(parent, "Bellum")
	workdir := t.TempDir()
	iconData, err := os.ReadFile(filepath.Join("..", "..", "packages", "launcher_1_256x256x32.png"))
	if err != nil {
		t.Fatal(err)
	}
	iconPath := filepath.Join(workdir, "packages", "launcher_1_256x256x32.png")
	if err := os.MkdirAll(filepath.Dir(iconPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(iconPath, iconData, 0644); err != nil {
		t.Fatal(err)
	}
	logger, _ := core.NewLogger("")
	fail := true
	mutate := func(_ core.RunMode, args []string, _ *core.Logger, _ string) error {
		if len(args) > 2 && args[1] == "winetricks" && args[2] == "-q" && fail {
			return errors.New("forced first attempt failure")
		}
		if len(args) == 2 && args[1] == "installer.exe" {
			runtime := filepath.Join(prefix, "drive_c", "Program Files (x86)", "Microsoft", "EdgeWebView", "Application", "1.0", "msedgewebview2.exe")
			if err := os.MkdirAll(filepath.Dir(runtime), 0700); err != nil {
				return err
			}
			return os.WriteFile(runtime, []byte("runtime"), 0600)
		}
		return nil
	}
	generate := func(launchers.LauncherConfig) error { return nil }
	config := InstallConfig{WINEPREFIX: prefix, ProtonPath: filepath.Join(workdir, "proton"), GPUType: "AMD", LauncherInstaller: "installer.exe", Workdir: workdir}
	boundaries := WorkflowBoundaries{Commands: transactionCommands{}, MutatePrefix: mutate, GenerateLauncher: generate}
	assetPaths := []string{filepath.Join(home, ".local", "bin", "Bellum"), filepath.Join(home, ".local", "share", "applications", "Bellum.desktop"), filepath.Join(home, "Desktop", "Bellum.desktop"), filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png")}
	for _, path := range assetPaths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("prior asset"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunInstallerWithBoundaries(config, logger, boundaries); err == nil {
		t.Fatal("expected first attempt to fail")
	}
	if _, err := os.Lstat(prefix); !os.IsNotExist(err) {
		t.Fatalf("failed first install left prefix behind: %v", err)
	}
	for _, path := range assetPaths {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "prior asset" {
			t.Fatalf("failed install changed prior launcher asset %s: %v", path, err)
		}
	}
	fail = false
	if err := RunInstallerWithBoundaries(config, logger, boundaries); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefix, manifestName)); err != nil {
		t.Fatalf("retry did not create prefix: %v", err)
	}
}

func TestLauncherGenerationFailureRestoresPriorAssets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	workdir := filepath.Join("..", "..")
	paths := []string{
		filepath.Join(home, ".local", "bin", "Bellum"),
		filepath.Join(home, ".local", "share", "applications", "Bellum.desktop"),
		filepath.Join(home, "Desktop", "Bellum.desktop"),
		filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("old:"+path), 0644); err != nil {
			t.Fatal(err)
		}
	}
	logger, _ := core.NewLogger("")
	err := generateLauncherWith(InstallConfig{WINEPREFIX: filepath.Join(t.TempDir(), "Bellum"), Workdir: workdir}, logger, func(launchers.LauncherConfig) error {
		_ = os.WriteFile(paths[0], []byte("partially replaced"), 0755)
		return errors.New("forced generation failure")
	})
	if err == nil {
		t.Fatal("expected generation error")
	}
	for _, path := range paths {
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.HasPrefix(string(got), "old:") {
			t.Fatalf("prior asset %s was not restored", path)
		}
	}
}

// TestRequestedWinetricksVerbsExistInProton checks the verbs against the
// winetricks bundled in an extracted Proton tree. It runs when
// BELLUM_PROTON_DIR points at one; run it whenever the Proton pin changes.
// Verified for proton-cachyos-11.0-20260703-slr (winetricks 20260125-next).
func TestRequestedWinetricksVerbsExistInProton(t *testing.T) {
	dir := os.Getenv("BELLUM_PROTON_DIR")
	if dir == "" {
		t.Skip("set BELLUM_PROTON_DIR to an extracted Proton tree to check winetricks verbs")
	}
	script, err := os.ReadFile(filepath.Join(dir, "protonfixes", "winetricks"))
	if err != nil {
		t.Fatal(err)
	}
	for _, verb := range append(append([]string{}, requiredWinetricksVerbs...), "win11", "remove_mono", "grabfullscreen=y", "windowmanagerdecorated=n", "mwo=disable") {
		if !strings.Contains(string(script), "w_metadata "+verb+" ") {
			t.Errorf("requested verb %q is not declared in Proton's winetricks", verb)
		}
	}
}

func TestResolvePrefixPathAppendsBellumOnce(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]string{
		"/games":             "/games/Bellum",
		"/games/":            "/games/Bellum",
		"/games/Bellum":      "/games/Bellum",
		"/games/Bellum/":     "/games/Bellum",
		"/games/NotBellum":   "/games/NotBellum/Bellum",
		"relative":           filepath.Join(cwd, "relative", "Bellum"),
		"  /padded/path  ":   "/padded/path/Bellum",
		"/games/../x/Bellum": "/x/Bellum",
	}
	for in, want := range tests {
		got, err := ResolvePrefixPath(in)
		if err != nil || got != want {
			t.Errorf("ResolvePrefixPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ResolvePrefixPath(" "); err == nil {
		t.Error("expected an error for an empty location")
	}
}

func TestValidatePrefixNeverCreatesDirectories(t *testing.T) {
	logger, _ := core.NewLogger("")
	prefix := filepath.Join(t.TempDir(), "Bellum")
	got, replace, err := validateWINEPREFIXWith(prefix, logger, osFiles{}, func(string) bool { return true })
	if err != nil || got != prefix || replace {
		t.Fatalf("validate = %q, %t, %v", got, replace, err)
	}
	if _, err := os.Lstat(prefix); !os.IsNotExist(err) {
		t.Fatalf("validation created the prefix: %v", err)
	}
}

func TestValidatePrefixClassifiesExistingFolders(t *testing.T) {
	logger, _ := core.NewLogger("")
	yes := func(string) bool { return true }
	no := func(string) bool { return false }

	empty := filepath.Join(t.TempDir(), "Bellum")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	if _, replace, err := validateWINEPREFIXWith(empty, logger, osFiles{}, yes); err != nil || replace {
		t.Fatalf("empty prefix: replace=%t err=%v", replace, err)
	}

	foreign := filepath.Join(t.TempDir(), "Bellum")
	if err := os.MkdirAll(filepath.Join(foreign, "stuff"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := validateWINEPREFIXWith(foreign, logger, osFiles{}, yes); err == nil || !strings.Contains(err.Error(), "not an empty folder") {
		t.Fatalf("foreign prefix: %v", err)
	}

	installed := filepath.Join(t.TempDir(), "Bellum")
	if err := os.Mkdir(installed, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(installed, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := validateWINEPREFIXWith(installed, logger, osFiles{}, yes); err == nil || !strings.Contains(err.Error(), "uninstaller") {
		t.Fatalf("installed prefix: %v", err)
	}

	incomplete := filepath.Join(t.TempDir(), "Bellum")
	if err := os.Mkdir(incomplete, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(incomplete, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if err := writeIncompleteMarker(incomplete, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if _, replace, err := validateWINEPREFIXWith(incomplete, logger, osFiles{}, yes); err != nil || !replace {
		t.Fatalf("incomplete prefix accepted: replace=%t err=%v", replace, err)
	}
	if _, _, err := validateWINEPREFIXWith(incomplete, logger, osFiles{}, no); err == nil {
		t.Fatal("declining to start over must cancel")
	}
	if _, err := os.Stat(filepath.Join(incomplete, manifestName)); err != nil {
		t.Fatalf("validation touched the unfinished install: %v", err)
	}
}

// A run killed after the prefix was created leaves the manifest and the
// install-incomplete marker behind. The next run must replace it and finish.
func TestInterruptedInstallIsReplacedOnRetry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	prefix := filepath.Join(t.TempDir(), "Bellum")
	workdir := filepath.Join("..", "..")
	if err := os.Mkdir(prefix, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(prefix, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if err := writeIncompleteMarker(prefix, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "leftover"), []byte("half-built"), 0600); err != nil {
		t.Fatal(err)
	}
	logger, _ := core.NewLogger("")
	mutate := func(_ core.RunMode, args []string, _ *core.Logger, _ string) error {
		if len(args) == 2 && args[1] == "installer.exe" {
			runtime := filepath.Join(prefix, "drive_c", "Program Files", "Microsoft", "EdgeWebView", "Application", "1.0", "msedgewebview2.exe")
			if err := os.MkdirAll(filepath.Dir(runtime), 0700); err != nil {
				return err
			}
			return os.WriteFile(runtime, []byte("runtime"), 0600)
		}
		return nil
	}
	boundaries := WorkflowBoundaries{Commands: transactionCommands{}, MutatePrefix: mutate, GenerateLauncher: func(launchers.LauncherConfig) error { return nil }}
	config := InstallConfig{WINEPREFIX: prefix, LauncherInstaller: "installer.exe", Workdir: workdir}

	if err := RunInstallerWithBoundaries(config, logger, boundaries); err == nil {
		t.Fatal("installing over an unfinished prefix without ReplaceIncomplete must fail")
	}
	if _, err := os.Stat(filepath.Join(prefix, "leftover")); err != nil {
		t.Fatalf("refused install still changed the prefix: %v", err)
	}

	config.ReplaceIncomplete = true
	if err := RunInstallerWithBoundaries(config, logger, boundaries); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefix, "leftover")); !os.IsNotExist(err) {
		t.Fatalf("old contents survived the restart: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefix, incompleteMarkerName)); err != nil {
		t.Fatalf("marker must remain until configuration finishes: %v", err)
	}
	if err := MarkInstallComplete(prefix); err != nil {
		t.Fatal(err)
	}
	if err := discardIncompleteInstallWith(prefix, osFiles{}); err == nil {
		t.Fatal("a finished install must never be discarded")
	}
}

func TestUninstallAcceptsUnfinishedInstall(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "Bellum")
	if err := os.Mkdir(prefix, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(prefix, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if err := validateBellumPrefix(prefix); err == nil {
		t.Fatal("a finished install without Wine markers must be refused")
	}
	if err := writeIncompleteMarker(prefix, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if err := validateBellumPrefix(prefix); err != nil {
		t.Fatalf("unfinished install should be removable: %v", err)
	}
}
