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

// MissingDependencyGuidance returns concrete package names and user actions.
// It never invokes a package manager; immutable systems are strictly guidance-only.
func MissingDependencyGuidance(h Host, missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	pkgs := map[string]string{
		"arch":     "wine winetricks umu-launcher wget mesa-demos",
		"fedora":   "wine winetricks umu-launcher wget glx-utils",
		"debian":   "wine winetricks umu-launcher wget mesa-utils",
		"opensuse": "wine winetricks umu-launcher wget Mesa-demo-x",
	}
	pkg := pkgs[h.PackageFamily()]
	if h.Immutable {
		return fmt.Sprintf("Missing %s. This host (%s) is immutable; install dependencies through its supported host or container workflow. Bellum will not modify it automatically.", strings.Join(missing, ", "), h.ID)
	}
	if h.PackageManager == "" || h.PackageFamily() == "unknown" {
		return fmt.Sprintf("Missing %s. Distribution/package manager is unknown; install the required tools (%s) using your distribution's documented method.", strings.Join(missing, ", "), pkg)
	}
	packages := map[string]string{"arch": "pacman -S", "fedora": "dnf install", "debian": "apt install", "opensuse": "zypper install"}[h.PackageFamily()]
	return fmt.Sprintf("Missing %s. Install required host packages (%s) with: %s %s", strings.Join(missing, ", "), pkg, packages, pkg)
}
