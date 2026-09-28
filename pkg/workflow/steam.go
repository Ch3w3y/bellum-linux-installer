package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The Proton EasyAntiCheat Runtime is Steam app 1826330. Steam installs and
// updates it; Bellum only locates and checks it.
const (
	eacRuntimeAppID      = "1826330"
	eacRuntimeInstallDir = "Proton EasyAntiCheat Runtime"
	eacRuntimeInstallCmd = "steam steam://install/" + eacRuntimeAppID
)

// EACRuntime is a located Proton EasyAntiCheat Runtime.
type EACRuntime struct {
	Path string
	// Manifest is the Steam appmanifest proving Steam installed this copy.
	// It is empty when PROTON_EAC_RUNTIME points somewhere else.
	Manifest string
	FromEnv  bool
}

// steamRoots lists where Steam keeps its data: native installs (including the
// ~/.steam symlinks some distributions create), the Flatpak and the Snap
// (Ubuntu's steam snap keeps its data under ~/snap/steam/common).
func steamRoots(home string) []string {
	return append(nativeSteamRootCandidates(home),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
		filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"),
	)
}

// nativeSteamRootCandidates lists where a native (unsandboxed) Steam keeps
// its data, including the ~/.steam symlinks some distributions create.
func nativeSteamRootCandidates(home string) []string {
	return []string{
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "root"),
	}
}

var vdfPathRe = regexp.MustCompile(`"path"\s+"((?:[^"\\]|\\.)*)"`)

// steamLibraries returns every Steam library folder reachable from the known
// roots, deduplicated through symlinks, in discovery order.
func steamLibraries(home string, files FileStore) []string {
	var libraries []string
	seen := map[string]bool{}
	add := func(dir string) {
		if !isDirWith(filepath.Join(dir, "steamapps"), files) {
			return
		}
		key := dir
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			key = resolved
		}
		if seen[key] {
			return
		}
		seen[key] = true
		libraries = append(libraries, dir)
	}
	for _, root := range steamRoots(home) {
		add(root)
		for _, vdf := range []string{
			filepath.Join(root, "steamapps", "libraryfolders.vdf"),
			filepath.Join(root, "config", "libraryfolders.vdf"),
		} {
			data, err := files.ReadFile(vdf)
			if err != nil {
				continue
			}
			for _, m := range vdfPathRe.FindAllStringSubmatch(string(data), -1) {
				add(strings.ReplaceAll(m[1], `\\`, `\`))
			}
		}
	}
	return libraries
}

var acfValueRe = regexp.MustCompile(`"(installdir|StateFlags)"\s+"([^"]*)"`)

// findEACRuntime locates the runtime: PROTON_EAC_RUNTIME first, then every
// Steam library that has app 1826330's appmanifest.
func findEACRuntime(home, override string, files FileStore) (EACRuntime, error) {
	if override != "" {
		if !isDirWith(override, files) {
			return EACRuntime{}, fmt.Errorf("PROTON_EAC_RUNTIME is set to %q, which is not a folder. Unset it, or point it at the Proton EasyAntiCheat Runtime folder", override)
		}
		return EACRuntime{Path: override, FromEnv: true}, nil
	}
	libraries := steamLibraries(home, files)
	for _, lib := range libraries {
		manifest := filepath.Join(lib, "steamapps", "appmanifest_"+eacRuntimeAppID+".acf")
		data, err := files.ReadFile(manifest)
		if err != nil {
			continue
		}
		installDir, flags := eacRuntimeInstallDir, ""
		for _, m := range acfValueRe.FindAllStringSubmatch(string(data), -1) {
			switch m[1] {
			case "installdir":
				installDir = m[2]
			case "StateFlags":
				flags = m[2]
			}
		}
		path := filepath.Join(lib, "steamapps", "common", installDir)
		if !isDirWith(path, files) {
			continue
		}
		// StateFlags bit 4 is "fully installed"; anything else means Steam is
		// still downloading or updating it.
		if n, err := strconv.Atoi(flags); err == nil && n&4 == 0 {
			return EACRuntime{}, fmt.Errorf("Steam has not finished installing or updating the Proton EasyAntiCheat Runtime in %s. Let Steam finish, then run the installer again", lib)
		}
		return EACRuntime{Path: path, Manifest: manifest}, nil
	}
	if len(libraries) == 0 {
		return EACRuntime{}, fmt.Errorf("No Steam library was found. Install Steam, sign in, then install the Proton EasyAntiCheat Runtime with: %s", eacRuntimeInstallCmd)
	}
	return EACRuntime{}, fmt.Errorf("The Proton EasyAntiCheat Runtime is not installed in any Steam library (%s). Install it with: %s", strings.Join(libraries, ", "), eacRuntimeInstallCmd)
}

// eacRuntimePath is the runtime path written into launch_vars.env: the
// discovered runtime, or the default native location if discovery fails.
func eacRuntimePath() string {
	home, _ := os.UserHomeDir()
	if rt, err := findEACRuntime(home, os.Getenv("PROTON_EAC_RUNTIME"), DefaultBoundaries.Files); err == nil {
		return rt.Path
	}
	return filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", eacRuntimeInstallDir)
}
