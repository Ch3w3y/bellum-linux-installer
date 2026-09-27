package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mkdirs(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// installEAC fakes Steam's install of app 1826330 into a library folder.
func installEAC(t *testing.T, library, stateFlags string) string {
	t.Helper()
	writeFile(t, filepath.Join(library, "steamapps", "appmanifest_1826330.acf"),
		"\"AppState\"\n{\n\t\"appid\"\t\t\"1826330\"\n\t\"StateFlags\"\t\t\""+stateFlags+"\"\n\t\"installdir\"\t\t\"Proton EasyAntiCheat Runtime\"\n}\n")
	runtime := filepath.Join(library, "steamapps", "common", "Proton EasyAntiCheat Runtime")
	mkdirs(t, runtime)
	return runtime
}

func TestFindEACRuntimeNativeSteam(t *testing.T) {
	home := t.TempDir()
	want := installEAC(t, filepath.Join(home, ".local", "share", "Steam"), "4")
	got, err := findEACRuntime(home, "", osFiles{})
	if err != nil || got.Path != want || got.Manifest == "" {
		t.Fatalf("got %+v, %v; want %s", got, err, want)
	}
}

func TestFindEACRuntimeFlatpakSteam(t *testing.T) {
	home := t.TempDir()
	want := installEAC(t, filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"), "4")
	got, err := findEACRuntime(home, "", osFiles{})
	if err != nil || got.Path != want {
		t.Fatalf("got %+v, %v; want %s", got, err, want)
	}
}

func TestFindEACRuntimeSecondaryLibrary(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".local", "share", "Steam")
	mkdirs(t, filepath.Join(root, "steamapps"))
	secondary := filepath.Join(t.TempDir(), "Games", "SteamLibrary")
	writeFile(t, filepath.Join(root, "steamapps", "libraryfolders.vdf"), `"libraryfolders"
{
	"0"
	{
		"path"		"`+root+`"
		"apps" { "228980" "1" }
	}
	"1"
	{
		"path"		"`+secondary+`"
		"apps" { "1826330" "1" }
	}
}
`)
	want := installEAC(t, secondary, "4")
	got, err := findEACRuntime(home, "", osFiles{})
	if err != nil || got.Path != want {
		t.Fatalf("got %+v, %v; want %s", got, err, want)
	}
}

func TestSteamLibrariesDeduplicateSymlinkedRoots(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".local", "share", "Steam")
	mkdirs(t, filepath.Join(root, "steamapps"), filepath.Join(home, ".steam"))
	if err := os.Symlink(root, filepath.Join(home, ".steam", "steam")); err != nil {
		t.Fatal(err)
	}
	if libs := steamLibraries(home, osFiles{}); len(libs) != 1 {
		t.Fatalf("expected one library, got %v", libs)
	}
}

func TestFindEACRuntimeReportsFixCommand(t *testing.T) {
	home := t.TempDir()
	if _, err := findEACRuntime(home, "", osFiles{}); err == nil || !strings.Contains(err.Error(), "steam steam://install/1826330") {
		t.Fatalf("no Steam: %v", err)
	}
	mkdirs(t, filepath.Join(home, ".local", "share", "Steam", "steamapps"))
	if _, err := findEACRuntime(home, "", osFiles{}); err == nil || !strings.Contains(err.Error(), "steam steam://install/1826330") {
		t.Fatalf("Steam without the runtime: %v", err)
	}
}

func TestFindEACRuntimeWaitsForSteamUpdate(t *testing.T) {
	home := t.TempDir()
	installEAC(t, filepath.Join(home, ".local", "share", "Steam"), "1026")
	if _, err := findEACRuntime(home, "", osFiles{}); err == nil || !strings.Contains(err.Error(), "not finished") {
		t.Fatalf("expected an update-in-progress error, got %v", err)
	}
}

func TestFindEACRuntimeHonoursOverride(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(t.TempDir(), "eac")
	mkdirs(t, custom)
	got, err := findEACRuntime(home, custom, osFiles{})
	if err != nil || got.Path != custom || !got.FromEnv {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := findEACRuntime(home, filepath.Join(custom, "missing"), osFiles{}); err == nil {
		t.Fatal("a missing override must be reported")
	}
}
