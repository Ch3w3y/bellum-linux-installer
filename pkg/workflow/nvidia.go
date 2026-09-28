package workflow

import (
	"fmt"
	"strconv"
	"strings"

	"bellum-installer/pkg/core"
)

// NVIDIA prechecks warn, never block. Each warning names the exact fix for
// the detected distribution. The commands are shown to the user only; the
// installer never runs them.

// minNVIDIADriver is the oldest proprietary/open NVIDIA driver the pinned
// Proton's DXVK supports. The pinned proton-cachyos-11.0-20260703 bundles
// DXVK v3.0.2, whose driver-support table lists 575.51.02 as the NVIDIA
// minimum (vkd3d-proton needs 535 or newer, which this covers). Revisit when
// the Proton pin moves to a new DXVK major.
const minNVIDIADriver = "575.51.02"

// rtx50RegressedBranch is the driver branch with community-reported
// Blackwell regressions under Proton (595.71.05 failing Vulkan swapchain
// creation where 595.58.03 worked).
const rtx50RegressedBranch = 595

// nvidiaFixes holds the distro-specific commands for each NVIDIA problem.
type nvidiaFixes struct {
	name    string // how the distro is named in the message
	driver  string // installs the proprietary driver (replaces nouveau/NVK)
	upgrade string // updates an installed driver
	vulkan  string // installs the NVIDIA Vulkan ICD (64- and 32-bit)
	modeset string // enables nvidia-drm.modeset=1
}

// Reusable kernel-parameter fixes.
const (
	modprobeModeset = "echo 'options nvidia-drm modeset=1' | sudo tee /etc/modprobe.d/nvidia-drm-modeset.conf"
	ostreeModeset   = "rpm-ostree kargs --append-if-missing=nvidia-drm.modeset=1"
)

// nvidiaFixesFor picks the fix commands from os-release identity. Specific
// distributions come before their family.
func nvidiaFixesFor(o core.OSInfo) nvidiaFixes {
	id := o.ID
	immutable := o.Immutable.Normalize() == core.TriYes
	switch {
	case id == "bazzite":
		return nvidiaFixes{
			name:    "Bazzite",
			driver:  "rebase to a Bazzite NVIDIA image, e.g. `rpm-ostree rebase ostree-image-signed:docker://ghcr.io/ublue-os/bazzite-nvidia-open:stable` (see Bazzite's docs for your image)",
			upgrade: "`ujust update`, then reboot",
			vulkan:  "rebase to a Bazzite NVIDIA image (the Vulkan driver ships with it)",
			modeset: "`" + ostreeModeset + "`, then reboot",
		}
	case id == "steamos":
		return nvidiaFixes{
			name:    "SteamOS",
			driver:  "SteamOS doesn't ship NVIDIA's driver; use a distribution with NVIDIA support (for example Bazzite's NVIDIA images)",
			upgrade: "use a distribution with NVIDIA support",
			vulkan:  "use a distribution with NVIDIA support",
			modeset: "use a distribution with NVIDIA support",
		}
	case id == "pop":
		return nvidiaFixes{
			name:    "Pop!_OS",
			driver:  "`sudo apt install system76-driver-nvidia`, then reboot",
			upgrade: "`sudo apt update && sudo apt full-upgrade`, then reboot",
			vulkan:  "`sudo apt install system76-driver-nvidia`",
			modeset: "`sudo kernelstub -a nvidia-drm.modeset=1`, then reboot",
		}
	case id == "debian":
		return nvidiaFixes{
			name:    "Debian",
			driver:  "enable the `non-free` and `non-free-firmware` components, then `sudo apt install nvidia-driver firmware-misc-nonfree` and reboot",
			upgrade: "`sudo apt update && sudo apt full-upgrade` (newer branches come from backports), then reboot",
			vulkan:  "`sudo dpkg --add-architecture i386 && sudo apt update && sudo apt install nvidia-vulkan-icd nvidia-vulkan-icd:i386`",
			modeset: "`" + modprobeModeset + " && sudo update-initramfs -u`, then reboot",
		}
	}
	switch o.Family.Normalize() {
	case core.OSArch:
		name := "Arch"
		if id == "manjaro" {
			return nvidiaFixes{
				name:    "Manjaro",
				driver:  "`sudo mhwd -a pci nonfree 0300`, then reboot",
				upgrade: "`sudo pacman -Syu`, then reboot",
				vulkan:  "`sudo pacman -S --needed nvidia-utils lib32-nvidia-utils`",
				modeset: "add `nvidia-drm.modeset=1` to GRUB_CMDLINE_LINUX_DEFAULT in /etc/default/grub, run `sudo update-grub`, then reboot",
			}
		}
		if id == "cachyos" {
			name = "CachyOS"
		}
		return nvidiaFixes{
			name:    name,
			driver:  "`sudo pacman -S --needed nvidia-open-dkms nvidia-utils lib32-nvidia-utils`, then reboot",
			upgrade: "`sudo pacman -Syu`, then reboot",
			vulkan:  "`sudo pacman -S --needed nvidia-utils lib32-nvidia-utils`",
			modeset: "`" + modprobeModeset + " && sudo mkinitcpio -P`, then reboot",
		}
	case core.OSFedora:
		if immutable {
			return nvidiaFixes{
				name:    "Fedora Atomic",
				driver:  "enable RPM Fusion, then `rpm-ostree install akmod-nvidia xorg-x11-drv-nvidia xorg-x11-drv-nvidia-libs.i686` and reboot",
				upgrade: "`rpm-ostree upgrade`, then reboot",
				vulkan:  "`rpm-ostree install xorg-x11-drv-nvidia-libs xorg-x11-drv-nvidia-libs.i686`, then reboot",
				modeset: "`" + ostreeModeset + "`, then reboot",
			}
		}
		return nvidiaFixes{
			name:    "Fedora",
			driver:  "enable RPM Fusion (nonfree), then `sudo dnf install akmod-nvidia xorg-x11-drv-nvidia-cuda` and reboot once the module has built",
			upgrade: "`sudo dnf upgrade --refresh`, then reboot",
			vulkan:  "`sudo dnf install xorg-x11-drv-nvidia-libs xorg-x11-drv-nvidia-libs.i686`",
			modeset: "`sudo grubby --update-kernel=ALL --args=nvidia-drm.modeset=1`, then reboot",
		}
	case core.OSDebian:
		name := "Ubuntu"
		if id == "linuxmint" || id == "mint" {
			name = "Linux Mint"
		}
		return nvidiaFixes{
			name:    name,
			driver:  "`sudo ubuntu-drivers install`, then reboot",
			upgrade: "`sudo apt update && sudo apt full-upgrade`, or a newer branch with `sudo ubuntu-drivers install`, then reboot",
			vulkan:  "reinstall the driver with `sudo ubuntu-drivers install` (it provides the Vulkan driver, `libnvidia-gl-<branch>`)",
			modeset: "`" + modprobeModeset + " && sudo update-initramfs -u`, then reboot",
		}
	case core.OSOpenSUSE:
		return nvidiaFixes{
			name:    "openSUSE",
			driver:  "add NVIDIA's repository, then `sudo zypper install nvidia-open-driver-G06-signed-kmp-default nvidia-video-G06 nvidia-gl-G06` and reboot",
			upgrade: "`sudo zypper refresh && sudo zypper update` (Tumbleweed: `sudo zypper dup`), then reboot",
			vulkan:  "`sudo zypper install nvidia-gl-G06 nvidia-gl-G06-32bit`",
			modeset: "`" + modprobeModeset + " && sudo dracut -f`, then reboot",
		}
	}
	return nvidiaFixes{
		name:    "your distribution",
		driver:  "install NVIDIA's proprietary or open-kernel driver with your distribution's documented method, then reboot",
		upgrade: "update the driver with your distribution's package manager, then reboot",
		vulkan:  "install your distribution's NVIDIA Vulkan driver (64- and 32-bit)",
		modeset: "add `nvidia-drm.modeset=1` to the kernel command line, then reboot",
	}
}

// compareVersions compares dotted numeric versions field by field.
// Unparseable fields compare as 0.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// usesNVIDIA reports whether the active GPU, or the loaded driver, is NVIDIA.
// A proprietary module loaded on a hybrid system whose active renderer is
// another vendor's (association "no") gets no NVIDIA advice.
func usesNVIDIA(p core.Platform) bool {
	if p.GPU.Vendor == core.GPUNVIDIA {
		return true
	}
	if p.GPU.Vendor != core.GPUUnknown && p.GPU.Vendor != "" {
		return false
	}
	switch p.NVIDIA.Kernel {
	case core.NVIDIAKernelProprietary, core.NVIDIAKernelOpen:
		return p.NVIDIA.Association.Normalize() != core.TriNo
	}
	return false
}

// NVIDIAWarnings returns the NVIDIA driver problems for the detected
// platform, each with its fix. It returns nothing on non-NVIDIA systems.
func NVIDIAWarnings(p core.Platform) []string {
	if !usesNVIDIA(p) {
		return nil
	}
	fix := nvidiaFixesFor(p.OS)
	nv := p.NVIDIA
	var warnings []string

	openSource := nv.Kernel == core.NVIDIAKernelNouveau ||
		nv.Userspace == core.NVIDIAUserspaceNouveau || nv.Userspace == core.NVIDIAUserspaceNVK
	if openSource {
		warnings = append(warnings, fmt.Sprintf("The open-source nouveau/NVK driver is in use. It has no DLSS or NVAPI, and Bellum with Easy Anti-Cheat is untested on it. Install NVIDIA's driver on %s: %s.", fix.name, fix.driver))
		// Version, modeset and ICD checks are about NVIDIA's own driver.
		return warnings
	}

	proprietary := nv.Kernel == core.NVIDIAKernelProprietary || nv.Kernel == core.NVIDIAKernelOpen
	if nv.Kernel == core.NVIDIAKernelNotObserved && p.GPU.Vendor == core.GPUNVIDIA {
		warnings = append(warnings, fmt.Sprintf("No NVIDIA kernel driver was detected for your NVIDIA GPU. Install NVIDIA's driver on %s: %s.", fix.name, fix.driver))
		return warnings
	}

	if nv.Version != "" && compareVersions(nv.Version, minNVIDIADriver) < 0 {
		warnings = append(warnings, fmt.Sprintf("NVIDIA driver %s is older than %s, the minimum for the pinned Proton's DXVK. Update it on %s: %s.", core.SanitizeField(nv.Version, 32), minNVIDIADriver, fix.name, fix.upgrade))
	}

	if p.GPU.Generation == "Blackwell" && nv.Version != "" {
		if major, err := strconv.Atoi(strings.SplitN(nv.Version, ".", 2)[0]); err == nil && major == rtx50RegressedBranch {
			warnings = append(warnings, fmt.Sprintf("NVIDIA driver %s is on the 595 branch, which has community-reported Proton regressions on RTX 50 cards (595.71.05 failing Vulkan swapchain creation where 595.58.03 worked). If you get shader-loading failures or a black screen, try 595.58.03 or the 590 branch.", core.SanitizeField(nv.Version, 32)))
		}
	}

	if proprietary && nv.Modeset.Normalize() == core.TriNo && (p.Session.Kind == core.SessionWayland || p.Session.Kind == core.SessionGamescope) {
		warnings = append(warnings, fmt.Sprintf("nvidia-drm.modeset is off on a Wayland session, which Wayland and XWayland need. Enable it on %s: %s.", fix.name, fix.modeset))
	}

	if proprietary && nv.Vulkan.NVIDIACandidate == core.NVIDIANotObserved && nv.Vulkan.ScanState == core.DiscoveryComplete {
		warnings = append(warnings, fmt.Sprintf("No NVIDIA Vulkan driver (nvidia_icd.json) was found, and Proton needs Vulkan. Install it on %s: %s.", fix.name, fix.vulkan))
	}
	return warnings
}
