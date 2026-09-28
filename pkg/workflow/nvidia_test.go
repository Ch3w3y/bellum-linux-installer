package workflow

import (
	"strings"
	"testing"

	"bellum-installer/pkg/core"
)

func nvidiaPlatform(osID string, family core.OSFamily, mut func(*core.Platform)) core.Platform {
	p := core.Platform{
		OS:  core.OSInfo{ID: osID, Family: family, Immutable: core.TriNo},
		GPU: core.GPUCapabilities{Vendor: core.GPUNVIDIA, Generation: "Ada", NVAPI: true, DLSS: true},
		NVIDIA: core.NVIDIAState{
			Kernel: core.NVIDIAKernelProprietary, Userspace: core.NVIDIAUserspaceNVIDIA,
			Association: core.TriYes, Version: "580.82.09", Modeset: core.TriYes,
			Vulkan: core.VulkanInfo{ScanState: core.DiscoveryComplete, NVIDIACandidate: core.NVIDIAPresent},
		},
		Session: core.SessionInfo{Kind: core.SessionWayland, GameMode: core.TriNo},
	}
	if mut != nil {
		mut(&p)
	}
	return p
}

func TestNVIDIAWarningsHealthyDriverIsQuiet(t *testing.T) {
	if w := NVIDIAWarnings(nvidiaPlatform("fedora", core.OSFedora, nil)); len(w) != 0 {
		t.Fatalf("unexpected warnings: %v", w)
	}
	// Non-NVIDIA systems never get NVIDIA advice, even with a module loaded
	// for an inactive adapter.
	amd := nvidiaPlatform("fedora", core.OSFedora, func(p *core.Platform) {
		p.GPU = core.GPUCapabilities{Vendor: core.GPUAMD, Generation: "RDNA3"}
		p.NVIDIA.Version = "470.1"
		p.NVIDIA.Association = core.TriNo
	})
	if w := NVIDIAWarnings(amd); len(w) != 0 {
		t.Fatalf("AMD system got NVIDIA warnings: %v", w)
	}
}

func TestNVIDIAWarningsPerProblemAndDistro(t *testing.T) {
	for _, tc := range []struct {
		name   string
		osID   string
		family core.OSFamily
		mut    func(*core.Platform)
		want   []string
	}{
		{"nouveau-fedora", "fedora", core.OSFedora, func(p *core.Platform) {
			p.NVIDIA.Kernel, p.NVIDIA.Userspace = core.NVIDIAKernelNouveau, core.NVIDIAUserspaceNouveau
		}, []string{"nouveau/NVK", "akmod-nvidia", "RPM Fusion"}},
		{"nvk-arch", "arch", core.OSArch, func(p *core.Platform) {
			p.NVIDIA.Kernel, p.NVIDIA.Userspace = core.NVIDIAKernelNotObserved, core.NVIDIAUserspaceNVK
			p.GPU.Renderer = "NVK AD104"
		}, []string{"nouveau/NVK", "nvidia-open-dkms"}},
		{"old-ubuntu", "ubuntu", core.OSDebian, func(p *core.Platform) { p.NVIDIA.Version = "550.54.14" }, []string{"550.54.14 is older than 575.51.02", "ubuntu-drivers"}},
		{"old-opensuse", "opensuse-tumbleweed", core.OSOpenSUSE, func(p *core.Platform) { p.NVIDIA.Version = "575.51.1" }, []string{"older than 575.51.02", "zypper dup"}},
		{"modeset-pop", "pop", core.OSDebian, func(p *core.Platform) { p.NVIDIA.Modeset = core.TriNo }, []string{"modeset is off", "kernelstub -a nvidia-drm.modeset=1"}},
		{"modeset-debian", "debian", core.OSDebian, func(p *core.Platform) { p.NVIDIA.Modeset = core.TriNo }, []string{"modeset is off", "update-initramfs"}},
		{"modeset-bazzite", "bazzite", core.OSFedora, func(p *core.Platform) { p.NVIDIA.Modeset = core.TriNo; p.OS.Immutable = core.TriYes }, []string{"rpm-ostree kargs"}},
		{"modeset-cachyos", "cachyos", core.OSArch, func(p *core.Platform) { p.NVIDIA.Modeset = core.TriNo }, []string{"CachyOS", "mkinitcpio"}},
		{"icd-missing-atomic", "fedora", core.OSFedora, func(p *core.Platform) {
			p.OS.Immutable = core.TriYes
			p.NVIDIA.Vulkan.NVIDIACandidate = core.NVIDIANotObserved
		}, []string{"nvidia_icd.json", "rpm-ostree install xorg-x11-drv-nvidia-libs"}},
		{"icd-missing-manjaro", "manjaro", core.OSArch, func(p *core.Platform) { p.NVIDIA.Vulkan.NVIDIACandidate = core.NVIDIANotObserved }, []string{"lib32-nvidia-utils"}},
		{"rtx50-595", "arch", core.OSArch, func(p *core.Platform) {
			p.GPU.Generation = "Blackwell"
			p.NVIDIA.Version = "595.71.05"
		}, []string{"595 branch", "595.58.03"}},
		{"no-driver-unknown-distro", "gentoo", core.OSUnknown, func(p *core.Platform) {
			p.NVIDIA.Kernel = core.NVIDIAKernelNotObserved
		}, []string{"No NVIDIA kernel driver", "documented method"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := NVIDIAWarnings(nvidiaPlatform(tc.osID, tc.family, tc.mut))
			joined := strings.Join(warnings, "\n")
			if len(warnings) == 0 {
				t.Fatal("no warnings")
			}
			for _, want := range tc.want {
				if !strings.Contains(joined, want) {
					t.Errorf("warnings lack %q:\n%s", want, joined)
				}
			}
		})
	}
}

// Mesa's NVK manifest beside NVIDIA's driver, with the kernel flavor
// unknown, is not proof that nouveau/NVK is in use.
func TestNVIDIAWarningsIgnoreNVKManifestBesideNVIDIA(t *testing.T) {
	p := nvidiaPlatform("fedora", core.OSFedora, func(p *core.Platform) {
		p.GPU.Renderer = "NVIDIA GeForce RTX 4070/PCIe/SSE2"
		p.NVIDIA.Kernel = core.NVIDIAKernelUnknown
		p.NVIDIA.Userspace = core.NVIDIAUserspaceNouveau
		p.NVIDIA.Version = "550.54.14"
	})
	warnings := strings.Join(NVIDIAWarnings(p), "\n")
	if strings.Contains(warnings, "nouveau/NVK") || !strings.Contains(warnings, "older than 575.51.02") {
		t.Fatalf("warnings:\n%s", warnings)
	}
}

func TestNVIDIAWarningsSkipUnknownEvidence(t *testing.T) {
	// Unknown modeset, a partial ICD scan and X11 all stay quiet: the checks
	// warn only on positive evidence of a problem.
	p := nvidiaPlatform("fedora", core.OSFedora, func(p *core.Platform) {
		p.NVIDIA.Modeset = core.TriUnknown
		p.NVIDIA.Vulkan = core.VulkanInfo{ScanState: core.DiscoveryPartial, NVIDIACandidate: core.NVIDIANotObserved}
	})
	if w := NVIDIAWarnings(p); len(w) != 0 {
		t.Fatalf("unexpected warnings: %v", w)
	}
	x11 := nvidiaPlatform("fedora", core.OSFedora, func(p *core.Platform) {
		p.NVIDIA.Modeset = core.TriNo
		p.Session.Kind = core.SessionX11
	})
	if w := NVIDIAWarnings(x11); len(w) != 0 {
		t.Fatalf("modeset warning on X11: %v", w)
	}
	// 595 on a non-Blackwell card is fine.
	ada := nvidiaPlatform("arch", core.OSArch, func(p *core.Platform) { p.NVIDIA.Version = "595.71.05" })
	if w := NVIDIAWarnings(ada); len(w) != 0 {
		t.Fatalf("unexpected warnings: %v", w)
	}
}

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"575.51.02", "575.51.02", 0},
		{"575.51.2", "575.51.02", 0},
		{"575.64", "575.51.02", 1},
		{"1000.0", "575.51.02", 1},
		{"99.1", "575.51.02", -1},
		{"575", "575.51.02", -1},
	} {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("compare(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
