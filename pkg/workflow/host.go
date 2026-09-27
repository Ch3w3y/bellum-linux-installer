package workflow

import (
	"bufio"
	"fmt"
	"strings"
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
	b, _ := files.ReadFile("/etc/os-release")
	values := map[string]string{}
	s := bufio.NewScanner(strings.NewReader(string(b)))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), "\"'")
		values[k] = v
	}
	h := Host{ID: strings.ToLower(values["ID"]), IDLike: strings.ToLower(values["ID_LIKE"]), VariantID: strings.ToLower(values["VARIANT_ID"])}
	h.Immutable = strings.Contains(h.ID, "steamos") || strings.Contains(h.ID, "bazzite") || strings.Contains(h.VariantID, "immutable") || strings.Contains(h.VariantID, "atomic")
	for _, bin := range []string{"pacman", "dnf", "apt-get", "zypper"} {
		if commands.LookPath(bin) != "" {
			h.PackageManager = bin
			break
		}
	}
	return h
}

func (h Host) PackageFamily() string {
	ids := strings.Fields(h.ID + " " + h.IDLike)
	for _, id := range ids {
		switch id {
		case "arch", "manjaro", "endeavouros":
			return "arch"
		case "fedora", "rhel", "centos":
			return "fedora"
		case "debian", "ubuntu", "linuxmint", "pop":
			return "debian"
		case "opensuse", "opensuse-leap", "opensuse-tumbleweed", "suse":
			return "opensuse"
		}
	}
	return "unknown"
}

// familyPackages lists the distribution-repository packages that provide the
// host tools, plus notes for tools a family does not ship in its main repos.
var familyPackages = map[string]struct {
	install, packages string
	notes             map[string]string
}{
	"arch": {
		install:  "sudo pacman -S",
		packages: "wine umu-launcher wget mesa-utils",
		notes: map[string]string{
			"umu-run":      "umu-launcher is in the [multilib] repository, which must be enabled.",
			"osslsigncode": "osslsigncode is in the AUR (for example: yay -S osslsigncode).",
		},
	},
	"fedora": {
		install:  "sudo dnf install",
		packages: "wine osslsigncode wget glx-utils",
		notes: map[string]string{
			"umu-run": "umu-launcher is not in the Fedora repositories; install it from " + umuReleasesURL + ".",
		},
	},
	"debian": {
		install:  "sudo apt install",
		packages: "wine osslsigncode wget mesa-utils",
		notes: map[string]string{
			"umu-run": "umu-launcher is not in the Debian/Ubuntu repositories; install the .deb from " + umuReleasesURL + ".",
		},
	},
	"opensuse": {
		install:  "sudo zypper install",
		packages: "wine osslsigncode wget Mesa-demo-x",
		notes: map[string]string{
			"umu-run": "umu-launcher is in the openSUSE 'games' OBS repository (https://build.opensuse.org/package/show/games/umu-launcher).",
		},
	},
}

const umuReleasesURL = "https://github.com/Open-Wine-Components/umu-launcher/releases"

// MissingDependencyGuidance returns concrete package names and user actions.
// It never invokes a package manager; immutable systems are strictly guidance-only.
func MissingDependencyGuidance(h Host, missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	list := strings.Join(missing, ", ")
	if h.Immutable {
		return fmt.Sprintf("Missing %s. This host (%s) is immutable; install dependencies through its supported host or container workflow. Bellum will not modify it automatically.", list, h.ID)
	}
	family, ok := familyPackages[h.PackageFamily()]
	if !ok {
		return fmt.Sprintf("Missing %s. Distribution/package manager is unknown; install wine, umu-launcher (%s), osslsigncode, wget, and glxinfo using your distribution's documented method.", list, umuReleasesURL)
	}
	msg := fmt.Sprintf("Missing %s. Install required host packages with: %s %s", list, family.install, family.packages)
	for _, tool := range missing {
		if note := family.notes[tool]; note != "" {
			msg += " " + note
		}
	}
	return msg
}
