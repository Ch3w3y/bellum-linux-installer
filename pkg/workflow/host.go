package workflow

import (
	"fmt"
	"strings"

	"bellum-installer/pkg/core"
)

// Host describes the Linux distribution and package tools available to the user.
type Host struct {
	ID, IDLike, VariantID string
	Immutable             bool
	PackageManager        string
}

func DetectHost(files FileStore, commands CommandRunner) Host {
	if files == nil {
		files = DefaultBoundaries.Files
	}
	if commands == nil {
		commands = DefaultBoundaries.Commands
	}
	// OS identity parsing is owned by core; this adapter preserves the
	// historical Host shape, package-manager lookup and guidance output.
	b, _ := files.ReadFile("/etc/os-release")
	id, idLike, variant, _, _, idLikeTokens, _ := core.ParseOSRelease(b)
	ostreeBooted := false
	if _, err := files.Stat("/run/ostree-booted"); err == nil {
		ostreeBooted = true
	}
	immutable, _ := core.ClassifyImmutable(id, variant, core.ClassifyOSFamily(id, idLikeTokens), ostreeBooted)
	h := Host{ID: id, IDLike: idLike, VariantID: variant}
	h.Immutable = immutable == core.TriYes
	for _, bin := range []string{"pacman", "dnf", "apt-get", "zypper"} {
		if commands.LookPath(bin) != "" {
			h.PackageManager = bin
			break
		}
	}
	return h
}

// PackageFamily is the package-manager family, classified by core so the
// guidance and platform detection agree (CachyOS, Nobara, Pop!_OS and the
// other derivatives included).
func (h Host) PackageFamily() string {
	return string(core.ClassifyOSFamily(h.ID, strings.Fields(h.IDLike)))
}

// familyPackages maps each required tool to the distribution package that
// provides it. A tool with no package there is covered by a note instead.
var familyPackages = map[string]struct {
	install  string
	packages map[string]string
	notes    map[string]string
}{
	"arch": {
		install:  "sudo pacman -S --needed",
		packages: map[string]string{"python3": "python", "flock": "util-linux", "glxinfo": "mesa-utils"},
	},
	"fedora": {
		install:  "sudo dnf install",
		packages: map[string]string{"python3": "python3", "flock": "util-linux", "glxinfo": "glx-utils"},
	},
	"debian": {
		install:  "sudo apt install",
		packages: map[string]string{"python3": "python3", "flock": "util-linux", "glxinfo": "mesa-utils"},
	},
	"opensuse": {
		install:  "sudo zypper install",
		packages: map[string]string{"python3": "python3", "flock": "util-linux", "glxinfo": "Mesa-demo-x"},
	},
}

// MissingDependencyGuidance names exactly the missing tools and one command
// that installs the ones the distribution packages. It never invokes a
// package manager; immutable systems get guidance only.
func MissingDependencyGuidance(h Host, missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	list := strings.Join(missing, ", ")
	if h.Immutable {
		return fmt.Sprintf("Missing %s. This host (%s) is immutable; install them through its supported host or container workflow (for example a Distrobox or Flatpak). Bellum will not modify it automatically.", list, h.ID)
	}
	family, ok := familyPackages[h.PackageFamily()]
	if !ok {
		return fmt.Sprintf("Missing %s. Distribution/package manager is unknown; install python3 (3.10 or newer) and util-linux (for flock) using your distribution's documented method.", list)
	}
	msg := fmt.Sprintf("Missing %s.", list)
	var pkgs []string
	for _, tool := range missing {
		if pkg := family.packages[tool]; pkg != "" {
			pkgs = append(pkgs, pkg)
		}
	}
	if len(pkgs) > 0 {
		msg += fmt.Sprintf(" Install with: `%s %s`.", family.install, strings.Join(pkgs, " "))
	}
	for _, tool := range missing {
		if note := family.notes[tool]; note != "" {
			msg += " " + note
		}
	}
	return msg
}
