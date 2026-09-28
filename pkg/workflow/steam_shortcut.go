package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"bellum-installer/pkg/core"
	"bellum-installer/pkg/steamvdf"
)

// The opt-in Steam shortcut: Bellum as a non-Steam game, so it can be
// started from Game Mode and gets Steam Input (the only way the Deck's and
// Steam Machine's built-in controls reach the game as a gamepad).
//
// Steam rewrites shortcuts.vdf from memory when it exits, so the file is
// only touched while Steam is closed, re-checked just before the write. The
// original is backed up, the new file is written next to it and renamed into
// place, and anything unexpected (a symlink, another owner, a file Steam
// didn't write) stops the change instead of guessing.

const steamShortcutName = "Bellum"

// errSteamRunning means Steam is running, or its state couldn't be proven.
var errSteamRunning = errors.New("Steam is running (or its state could not be checked); close Steam completely, then try again")

// steamShortcutHost holds the host effects of the shortcut writer.
type steamShortcutHost struct {
	// SteamRunning re-checks the Steam client at action time.
	SteamRunning func() core.TriState
	Now          func() time.Time
	EUID         int
}

var defaultSteamShortcutHost = steamShortcutHost{
	SteamRunning: func() core.TriState {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return core.SteamRunningNow(ctx)
	},
	Now:  time.Now,
	EUID: os.Geteuid(),
}

// bellumShortcut is the entry for the Bellum wrapper in home.
func bellumShortcut(home string) steamvdf.Shortcut {
	bin := filepath.Join(home, ".local", "bin")
	return steamvdf.Shortcut{
		AppName:  steamShortcutName,
		Exe:      filepath.Join(bin, "Bellum"),
		StartDir: bin,
		Icon:     filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png"),
	}
}

// nativeSteamRoots lists the native Steam data roots, resolved and
// deduplicated. Flatpak and Snap Steam are sandboxed and cannot start
// ~/.local/bin/Bellum, so they are never given a shortcut.
func nativeSteamRoots(home string) []string {
	var roots []string
	seen := map[string]bool{}
	for _, r := range []string{
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "root"),
	} {
		resolved, err := filepath.EvalSymlinks(r)
		if err != nil || seen[resolved] {
			continue
		}
		if info, err := os.Stat(filepath.Join(resolved, "userdata")); err != nil || !info.IsDir() {
			continue
		}
		seen[resolved] = true
		roots = append(roots, resolved)
	}
	return roots
}

// steamID64Base converts a SteamID64 to the account ID used in userdata.
const steamID64Base = 76561197960265728

var mostRecentUserRe = regexp.MustCompile(`"(\d{17})"\s*\{[^{}]*"MostRecent"\s*"1"`)

// steamShortcutsFile finds the shortcuts.vdf of the Steam account to use:
// the only account on the machine, or the most recently signed-in one.
func steamShortcutsFile(home string) (string, error) {
	roots := nativeSteamRoots(home)
	if len(roots) == 0 {
		return "", fmt.Errorf("no native Steam install with a signed-in account was found (Flatpak and Snap Steam can't start Bellum's launcher directly)")
	}
	root := roots[0]
	names, err := os.ReadDir(filepath.Join(root, "userdata"))
	if err != nil {
		return "", err
	}
	var accounts []string
	for _, e := range names {
		if n, err := strconv.ParseUint(e.Name(), 10, 32); err == nil && n != 0 && e.IsDir() {
			accounts = append(accounts, e.Name())
		}
	}
	account := ""
	switch len(accounts) {
	case 0:
		return "", fmt.Errorf("no Steam account has signed in on this machine yet; start Steam and sign in once")
	case 1:
		account = accounts[0]
	default:
		data, err := readBoundedFile(filepath.Join(root, "config", "loginusers.vdf"), 1<<20)
		if err == nil {
			if m := mostRecentUserRe.FindSubmatch(data); m != nil {
				if id, err := strconv.ParseUint(string(m[1]), 10, 64); err == nil && id > steamID64Base {
					account = strconv.FormatUint(id-steamID64Base, 10)
				}
			}
		}
		found := false
		for _, a := range accounts {
			found = found || a == account
		}
		if !found {
			return "", fmt.Errorf("several Steam accounts are signed in on this machine and the most recent one couldn't be identified")
		}
	}
	return filepath.Join(root, "userdata", account, "config", "shortcuts.vdf"), nil
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s is larger than %d bytes", path, limit)
	}
	buf := make([]byte, info.Size())
	if _, err := f.ReadAt(buf, 0); err != nil && info.Size() > 0 {
		return nil, err
	}
	return buf, nil
}

// loadShortcuts reads and strictly parses shortcuts.vdf. A missing file
// yields an empty document; anything else unusual is an error.
func loadShortcuts(path string, euid int) (*steamvdf.Node, []byte, os.FileMode, error) {
	dirInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("Steam's config folder is missing: %w", err)
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, 0, fmt.Errorf("%s is not a plain folder", filepath.Dir(path))
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return steamvdf.NewDocument(), nil, 0600, nil
	}
	if err != nil {
		return nil, nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, 0, fmt.Errorf("%s is not a regular file", path)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != euid {
		return nil, nil, 0, fmt.Errorf("%s belongs to another user", path)
	}
	data, err := readBoundedFile(path, steamvdf.MaxFileSize)
	if err != nil {
		return nil, nil, 0, err
	}
	root, err := steamvdf.Parse(data)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("%s could not be read safely, so it was left unchanged: %w", path, err)
	}
	return root, data, info.Mode().Perm(), nil
}

// saveShortcuts backs up the original (when there is one) and atomically
// replaces path with root, re-checking that Steam is still closed.
func saveShortcuts(path string, root *steamvdf.Node, original []byte, mode os.FileMode, host steamShortcutHost) (string, error) {
	data, err := steamvdf.Encode(root)
	if err != nil {
		return "", err
	}
	if _, err := steamvdf.Parse(data); err != nil {
		return "", fmt.Errorf("refusing to write an unreadable shortcuts.vdf: %w", err)
	}
	backup := ""
	if original != nil {
		// O_EXCL never overwrites an earlier backup; a clash gets a suffix.
		var f *os.File
		for i := 0; f == nil; i++ {
			backup = fmt.Sprintf("%s.bellum-backup-%d", path, host.Now().Unix())
			if i > 0 {
				backup += fmt.Sprintf("-%d", i)
			}
			f, err = os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil && (!os.IsExist(err) || i >= 99) {
				return "", fmt.Errorf("back up shortcuts.vdf: %w", err)
			}
		}
		_, werr := f.Write(original)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return "", fmt.Errorf("back up shortcuts.vdf: %w", werr)
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".shortcuts.vdf.bellum-*")
	if err != nil {
		return backup, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return backup, err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return backup, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return backup, err
	}
	if err := tmp.Close(); err != nil {
		return backup, err
	}
	if host.SteamRunning() != core.TriNo {
		return backup, errSteamRunning
	}
	return backup, os.Rename(tmpName, path)
}

// addBellumSteamShortcut adds the Bellum entry. It reports the file it
// changed (empty when the entry was already there) and the backup path.
func addBellumSteamShortcut(home string, host steamShortcutHost) (path, backup string, err error) {
	if host.SteamRunning() != core.TriNo {
		return "", "", errSteamRunning
	}
	path, err = steamShortcutsFile(home)
	if err != nil {
		return "", "", err
	}
	root, original, mode, err := loadShortcuts(path, host.EUID)
	if err != nil {
		return "", "", err
	}
	if !steamvdf.AddShortcut(root, bellumShortcut(home)) {
		return "", "", nil
	}
	backup, err = saveShortcuts(path, root, original, mode, host)
	return path, backup, err
}

// removeBellumSteamShortcut removes the Bellum entry the installer added.
// It reports how many entries were removed.
func removeBellumSteamShortcut(home string, host steamShortcutHost) (int, error) {
	path, err := steamShortcutsFile(home)
	if err != nil {
		return 0, nil // no native Steam account: nothing was ever added
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return 0, nil
	}
	root, original, mode, err := loadShortcuts(path, host.EUID)
	if err != nil {
		return 0, err
	}
	s := bellumShortcut(home)
	if !steamvdf.HasShortcut(root, s.Exe) {
		return 0, nil
	}
	if host.SteamRunning() != core.TriNo {
		return 0, errSteamRunning
	}
	n := steamvdf.RemoveShortcuts(root, s.Exe, s.AppName)
	if n == 0 {
		return 0, nil
	}
	_, err = saveShortcuts(path, root, original, mode, host)
	return n, err
}

// OfferSteamShortcut asks, default No, whether to add Bellum to Steam, on
// platforms that should launch through Steam. With --yes it only explains
// the manual steps, since the answer's default is No. It reports whether
// the entry is now in Steam.
func OfferSteamShortcut(p core.Platform, logger *core.Logger) bool {
	return offerSteamShortcutWith(p, logger, core.AskBoolDefaultNo, core.AssumeYes, defaultSteamShortcutHost)
}

func offerSteamShortcutWith(p core.Platform, logger *core.Logger, ask func(string) bool, assumeYes bool, host steamShortcutHost) bool {
	if !core.SupportsGameModeIntegration(p) {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil || len(nativeSteamRoots(home)) == 0 {
		return false
	}
	if path, err := steamShortcutsFile(home); err == nil {
		if root, _, _, err := loadShortcuts(path, host.EUID); err == nil && steamvdf.HasShortcut(root, bellumShortcut(home).Exe) {
			logger.Info("[OK] Bellum is already in your Steam library")
			return true
		}
	}
	if assumeYes {
		return false
	}
	fmt.Println()
	fmt.Println("  Bellum can add itself to your Steam library, so you can start it in Game Mode")
	fmt.Println("  and use the built-in controls. Steam must be closed completely (in Desktop Mode,")
	fmt.Println("  Steam menu → Exit); its shortcuts file is backed up first.")
	if !ask(fmt.Sprintf("  %s?%s Add Bellum to Steam now? %s[y/N]%s ", core.ColorBoldCyan, core.ColorReset, core.ColorGrayBold, core.ColorReset)) {
		return false
	}
	path, backup, err := addBellumSteamShortcut(home, host)
	if err != nil {
		logger.Warn("Bellum was not added to Steam: " + err.Error() + ". Add it by hand with the steps below.")
		return false
	}
	switch {
	case path == "":
		logger.Info("[OK] Bellum is already in your Steam library")
	case backup != "":
		logger.Info(fmt.Sprintf("[OK] Added Bellum to Steam (%s; backup: %s)", path, filepath.Base(backup)))
	default:
		logger.Info(fmt.Sprintf("[OK] Added Bellum to Steam (%s)", path))
	}
	return true
}

// FinishAdvice is the finish screen's advice for playing through Steam and
// with a controller. It is empty on ordinary desktops without controllers.
func FinishAdvice(p core.Platform, inSteam bool) []string {
	steer := core.SupportsGameModeIntegration(p)
	pads := len(p.Controllers.Devices) > 0
	if !steer && !pads {
		return nil
	}
	var lines []string
	if steer {
		lines = append(lines, "To play in Game Mode and with the built-in controls, start Bellum from Steam:")
	} else {
		lines = append(lines, "Controller found. For the best controller support, start Bellum from Steam:")
	}
	if inSteam {
		lines = append(lines, "  Bellum is in your Steam library. Restart Steam to see it.")
	} else {
		lines = append(lines,
			"  1. In Desktop Mode, open Steam → Games → Add a Non-Steam Game to My Library…",
			"  2. Browse to ~/.local/bin/Bellum (show hidden files), add it.")
	}
	lines = append(lines,
		"  Don't force a Proton version on it in Properties → Compatibility: Bellum runs its own Proton.",
		"  Controller layout: a gamepad template with the right trackpad as a mouse (the launcher",
		"  needs a pointer to sign in and press Play); Steam + X opens the on-screen keyboard.")
	return lines
}
