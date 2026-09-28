package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bellum-installer/pkg/core"
	"bellum-installer/pkg/steamvdf"
)

func shortcutHost(state core.TriState) steamShortcutHost {
	return steamShortcutHost{
		SteamRunning: func() core.TriState { return state },
		Now:          func() time.Time { return time.Unix(1700000000, 0) },
		EUID:         os.Geteuid(),
	}
}

// steamAccount creates a native Steam root with one signed-in account and
// returns its shortcuts.vdf path.
func steamAccount(t *testing.T, home, account string) string {
	t.Helper()
	config := filepath.Join(home, ".local", "share", "Steam", "userdata", account, "config")
	mkdirs(t, config)
	return filepath.Join(config, "shortcuts.vdf")
}

func existingShortcuts(t *testing.T, path string) []byte {
	t.Helper()
	root := steamvdf.NewDocument()
	steamvdf.AddShortcut(root, steamvdf.Shortcut{AppName: "Other Game", Exe: "/usr/bin/other", StartDir: "/usr/bin"})
	data, err := steamvdf.Encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAddBellumSteamShortcut(t *testing.T) {
	home := t.TempDir()
	path := steamAccount(t, home, "12345")
	original := existingShortcuts(t, path)

	got, backup, err := addBellumSteamShortcut(home, shortcutHost(core.TriNo))
	if err != nil || got != path || backup == "" {
		t.Fatalf("add: %q %q %v", got, backup, err)
	}
	if saved, _ := os.ReadFile(backup); !bytes.Equal(saved, original) {
		t.Fatal("backup differs from the original")
	}
	data, _ := os.ReadFile(path)
	root, err := steamvdf.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !steamvdf.HasShortcut(root, filepath.Join(home, ".local", "bin", "Bellum")) || !steamvdf.HasShortcut(root, "/usr/bin/other") {
		t.Fatal("entries missing after add")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0644 {
		t.Fatalf("mode changed to %v", info.Mode().Perm())
	}
	// Idempotent: a second run changes nothing.
	if got, _, err := addBellumSteamShortcut(home, shortcutHost(core.TriNo)); err != nil || got != "" {
		t.Fatalf("second add: %q %v", got, err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".shortcuts.vdf.bellum-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temporary files left: %v", leftovers)
	}

	// The uninstaller removes exactly the Bellum entry.
	if n, err := removeBellumSteamShortcut(home, shortcutHost(core.TriNo)); err != nil || n != 1 {
		t.Fatalf("remove: %d %v", n, err)
	}
	data, _ = os.ReadFile(path)
	root, _ = steamvdf.Parse(data)
	if steamvdf.HasShortcut(root, filepath.Join(home, ".local", "bin", "Bellum")) || !steamvdf.HasShortcut(root, "/usr/bin/other") {
		t.Fatal("remove took the wrong entries")
	}
}

func TestAddBellumSteamShortcutCreatesTheFile(t *testing.T) {
	home := t.TempDir()
	path := steamAccount(t, home, "12345")
	if got, backup, err := addBellumSteamShortcut(home, shortcutHost(core.TriNo)); err != nil || got != path || backup != "" {
		t.Fatalf("add: %q %q %v", got, backup, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("new file: %v %v", info, err)
	}
}

func TestSteamShortcutRefusals(t *testing.T) {
	for name, setup := range map[string]func(t *testing.T, home, path string) steamShortcutHost{
		"steam running": func(t *testing.T, home, path string) steamShortcutHost {
			existingShortcuts(t, path)
			return shortcutHost(core.TriYes)
		},
		"steam state unknown": func(t *testing.T, home, path string) steamShortcutHost {
			existingShortcuts(t, path)
			return shortcutHost(core.TriUnknown)
		},
		"steam starts before the rename": func(t *testing.T, home, path string) steamShortcutHost {
			existingShortcuts(t, path)
			calls := 0
			h := shortcutHost(core.TriNo)
			h.SteamRunning = func() core.TriState {
				if calls++; calls > 1 {
					return core.TriYes
				}
				return core.TriNo
			}
			return h
		},
		"symlinked file": func(t *testing.T, home, path string) steamShortcutHost {
			target := filepath.Join(t.TempDir(), "elsewhere.vdf")
			existingShortcuts(t, target)
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			return shortcutHost(core.TriNo)
		},
		"unreadable content": func(t *testing.T, home, path string) steamShortcutHost {
			if err := os.WriteFile(path, []byte("\"shortcuts\" { text vdf }"), 0600); err != nil {
				t.Fatal(err)
			}
			return shortcutHost(core.TriNo)
		},
		"another owner": func(t *testing.T, home, path string) steamShortcutHost {
			existingShortcuts(t, path)
			h := shortcutHost(core.TriNo)
			h.EUID = os.Geteuid() + 1
			return h
		},
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			path := steamAccount(t, home, "12345")
			host := setup(t, home, path)
			before, _ := os.ReadFile(path)
			if _, _, err := addBellumSteamShortcut(home, host); err == nil {
				t.Fatal("expected a refusal")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("shortcuts.vdf changed despite the refusal")
			}
		})
	}
}

func TestSteamShortcutsFileAccountChoice(t *testing.T) {
	home := t.TempDir()
	if _, err := steamShortcutsFile(home); err == nil {
		t.Fatal("no Steam: expected an error")
	}
	mkdirs(t, filepath.Join(home, ".local", "share", "Steam", "userdata", "0"))
	if _, err := steamShortcutsFile(home); err == nil || !strings.Contains(err.Error(), "signed in") {
		t.Fatalf("anonymous only: %v", err)
	}
	steamAccount(t, home, "111")
	steamAccount(t, home, "222")
	if _, err := steamShortcutsFile(home); err == nil {
		t.Fatal("two accounts without loginusers.vdf: expected an error")
	}
	// SteamID64 76561197960265950 is account 222.
	loginusers := "\"users\"\n{\n\t\"76561197960265839\"\n\t{\n\t\t\"MostRecent\"\t\t\"0\"\n\t}\n\t\"76561197960265950\"\n\t{\n\t\t\"AccountName\"\t\t\"x\"\n\t\t\"MostRecent\"\t\t\"1\"\n\t}\n}\n"
	writeFile(t, filepath.Join(home, ".local", "share", "Steam", "config", "loginusers.vdf"), loginusers)
	got, err := steamShortcutsFile(home)
	if err != nil || !strings.Contains(got, filepath.Join("userdata", "222", "config")) {
		t.Fatalf("most recent: %q %v", got, err)
	}
}

func TestOfferSteamShortcut(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := steamAccount(t, home, "12345")
	existingShortcuts(t, path)
	logger, _ := core.NewLogger("")
	deck := core.Platform{OS: core.OSInfo{ID: "steamos"}, Hardware: core.HardwareInfo{SKU: core.SKUDeckLCD}}
	desktop := core.Platform{OS: core.OSInfo{ID: "fedora"}}
	never := func(string) bool { t.Fatal("asked"); return false }

	if offerSteamShortcutWith(desktop, logger, never, false, shortcutHost(core.TriNo)) {
		t.Fatal("offered on a desktop")
	}
	// --yes keeps the default No without asking.
	if offerSteamShortcutWith(deck, logger, never, true, shortcutHost(core.TriNo)) {
		t.Fatal("--yes added the shortcut")
	}
	if offerSteamShortcutWith(deck, logger, func(string) bool { return false }, false, shortcutHost(core.TriNo)) {
		t.Fatal("added after No")
	}
	if !offerSteamShortcutWith(deck, logger, func(string) bool { return true }, false, shortcutHost(core.TriNo)) {
		t.Fatal("not added after yes")
	}
	// Already present: reported without asking again.
	if !offerSteamShortcutWith(deck, logger, never, false, shortcutHost(core.TriNo)) {
		t.Fatal("existing entry not reported")
	}
}

func TestFinishAdvice(t *testing.T) {
	if lines := FinishAdvice(core.Platform{OS: core.OSInfo{ID: "arch"}}, false); lines != nil {
		t.Fatalf("desktop without pads: %v", lines)
	}
	deck := strings.Join(FinishAdvice(core.Platform{Hardware: core.HardwareInfo{SKU: core.SKUDeckOLED}}, false), "\n")
	for _, want := range []string{"Game Mode", "Add a Non-Steam Game", "~/.local/bin/Bellum", "Don't force a Proton", "trackpad", "Steam + X"} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck advice lacks %q:\n%s", want, deck)
		}
	}
	added := strings.Join(FinishAdvice(core.Platform{Hardware: core.HardwareInfo{SKU: core.SKUDeckOLED}}, true), "\n")
	if strings.Contains(added, "Add a Non-Steam Game") || !strings.Contains(added, "Restart Steam") {
		t.Errorf("advice after adding:\n%s", added)
	}
	pads := core.Platform{Controllers: core.ControllerInfo{Devices: []core.ControllerDevice{{Name: "Xbox Wireless Controller"}}}}
	if got := strings.Join(FinishAdvice(pads, false), "\n"); !strings.Contains(got, "Controller found") {
		t.Errorf("pad advice:\n%s", got)
	}
}

// The uninstaller finds the entry under every account, even after another
// account became the most recent one, and reports files it can't check.
func TestRemoveBellumSteamShortcutAllAccounts(t *testing.T) {
	home := t.TempDir()
	first := steamAccount(t, home, "111")
	second := steamAccount(t, home, "222")
	for _, path := range []string{first, second} {
		root := steamvdf.NewDocument()
		steamvdf.AddShortcut(root, bellumShortcut(home))
		data, _ := steamvdf.Encode(root)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := removeBellumSteamShortcut(home, shortcutHost(core.TriNo)); err != nil || n != 2 {
		t.Fatalf("removed %d, %v", n, err)
	}

	// An unreadable file is reported, not silently skipped.
	if err := os.WriteFile(first, []byte("not vdf"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := removeBellumSteamShortcut(home, shortcutHost(core.TriNo)); err == nil {
		t.Fatal("expected an error for an unreadable shortcuts.vdf")
	}
	// Steam running with an entry present is reported too.
	root := steamvdf.NewDocument()
	steamvdf.AddShortcut(root, bellumShortcut(home))
	data, _ := steamvdf.Encode(root)
	_ = os.WriteFile(second, data, 0600)
	_ = os.Remove(first)
	if _, err := removeBellumSteamShortcut(home, shortcutHost(core.TriYes)); err == nil {
		t.Fatal("expected errSteamRunning")
	}
}

func TestSteamShortcutsFileLowercaseMostRecent(t *testing.T) {
	home := t.TempDir()
	steamAccount(t, home, "111")
	steamAccount(t, home, "222")
	loginusers := "\"users\"\n{\n\t\"76561197960265950\"\n\t{\n\t\t\"mostrecent\"\t\t\"1\"\n\t}\n}\n"
	writeFile(t, filepath.Join(home, ".local", "share", "Steam", "config", "loginusers.vdf"), loginusers)
	if got, err := steamShortcutsFile(home); err != nil || !strings.Contains(got, filepath.Join("userdata", "222")) {
		t.Fatalf("got %q, %v", got, err)
	}
}
