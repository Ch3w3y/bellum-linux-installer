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

// familyPackages maps each required tool to the distribution package that
// provides it. A tool with no package there is covered by a note instead.
var familyPackages = map[string]struct {
	install  string
	packages map[string]string
	notes    map[string]string
}{
	"arch": {
		install:  "sudo pacman -S",
		packages: map[string]string{"umu-run": "umu-launcher", "wget": "wget", "glxinfo": "mesa-utils"},
		notes: map[string]string{
			"umu-run":      "umu-launcher is in the [multilib] repository, which must be enabled.",
			"osslsigncode": "osslsigncode is in the AUR (for example: yay -S osslsigncode).",
		},
	},
	"fedora": {
		install:  "sudo dnf install",
		packages: map[string]string{"osslsigncode": "osslsigncode", "wget": "wget", "glxinfo": "glx-utils"},
		notes: map[string]string{
			"umu-run": "umu-launcher is not in the Fedora repositories; install it from " + umuReleasesURL + ".",
		},
	},
	"debian": {
		install:  "sudo apt install",
		packages: map[string]string{"osslsigncode": "osslsigncode", "wget": "wget", "glxinfo": "mesa-utils"},
		notes: map[string]string{
			"umu-run": "umu-launcher is not in the Debian/Ubuntu repositories; install the .deb from " + umuReleasesURL + ".",
		},
	},
	"opensuse": {
		install:  "sudo zypper install",
		packages: map[string]string{"osslsigncode": "osslsigncode", "wget": "wget", "glxinfo": "Mesa-demo-x"},
		notes: map[string]string{
			"umu-run": "umu-launcher is in the openSUSE 'games' OBS repository (https://build.opensuse.org/package/show/games/umu-launcher).",
		},
	},
}

const umuReleasesURL = "https://github.com/Open-Wine-Components/umu-launcher/releases"

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
		return fmt.Sprintf("Missing %s. Distribution/package manager is unknown; install umu-launcher (%s), osslsigncode and wget using your distribution's documented method.", list, umuReleasesURL)
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
