package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GPUProbeBudget is the proposed total budget across the production GPU
// probe's subprocesses. No unbounded child is left after cancellation.
const GPUProbeBudget = 5 * time.Second

// ProductionGPUProbe runs the accepted step-1 acquisition chain (glxinfo,
// then lspci, then DRM sysfs vendor IDs) with cancellation and provenance.
// It only errors when every probe fails so callers degrade gracefully.
func ProductionGPUProbe(ctx context.Context) (GPUCapabilities, GPUProbeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, GPUProbeBudget)
	defer cancel()

	if renderer, ok, outcome := probeGlxinfo(ctx); ok {
		caps := ClassifyGPUCapabilities(renderer)
		caps.Renderer = renderer
		return caps, GPUProbeInfo{Source: ProbeRenderer, Renderer: SanitizeField(renderer, MaxRendererLen), DeviceAssociation: "known", Provenance: "step1:glxinfo"}, nil
	} else if ctx.Err() != nil {
		return GPUCapabilities{}, GPUProbeInfo{Source: ProbeUnknown, Provenance: "step1:cancelled"}, ctx.Err()
	} else {
		_ = outcome
	}

	if renderer, vendor, device, ok := probeLspci(ctx); ok {
		caps := ClassifyGPUCapabilities(renderer)
		caps.Renderer = renderer
		return caps, GPUProbeInfo{Source: ProbeLspci, Renderer: SanitizeField(renderer, MaxRendererLen), PCIVendor: vendor, PCIDevice: device, DeviceAssociation: "known", Provenance: "step1:lspci"}, nil
	} else if ctx.Err() != nil {
		return GPUCapabilities{}, GPUProbeInfo{Source: ProbeUnknown, Provenance: "step1:cancelled"}, ctx.Err()
	}

	if caps, vendor, ok := probeDRM(ctx); ok {
		caps.Renderer = SanitizeField(caps.Renderer, MaxRendererLen)
		return caps, GPUProbeInfo{Source: ProbeDRM, Renderer: caps.Renderer, PCIVendor: vendor, DeviceAssociation: "unknown", Provenance: "step1:drm"}, nil
	} else if ctx.Err() != nil {
		return GPUCapabilities{}, GPUProbeInfo{Source: ProbeUnknown, Provenance: "step1:cancelled"}, ctx.Err()
	}

	return GPUCapabilities{}, GPUProbeInfo{Source: ProbeUnknown, DeviceAssociation: "unknown", Provenance: "step1:none"}, fmt.Errorf("no GPU probe could identify the adapter")
}

func probeGlxinfo(ctx context.Context) (string, bool, error) {
	cmd := exec.CommandContext(ctx, "glxinfo", "-B")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", false, err
	}
	renderer := parseRenderer(string(output))
	if strings.TrimSpace(renderer) == "" {
		return "", false, fmt.Errorf("glxinfo output did not contain an OpenGL renderer")
	}
	return renderer, true, nil
}

func probeLspci(ctx context.Context) (renderer, vendor, device string, ok bool) {
	if LookPath("lspci") == "" {
		return "", "", "", false
	}
	cmd := exec.CommandContext(ctx, "lspci", "-nn")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", "", false
	}
	for _, line := range strings.Split(string(output), "\n") {
		if r, good := parseLspciRendererLine(line); good {
			vendor, device = lspciVendorDevice(line)
			return r, vendor, device, true
		}
	}
	return "", "", "", false
}

var lspciVendorDeviceRe = regexp.MustCompile(`\[([[:xdigit:]]{4}):([[:xdigit:]]{4})\]`)

func lspciVendorDevice(line string) (string, string) {
	m := lspciVendorDeviceRe.FindStringSubmatch(line)
	if m == nil {
		return "", ""
	}
	return strings.ToLower(m[1]), strings.ToLower(m[2])
}

func probeDRM(ctx context.Context) (GPUCapabilities, string, bool) {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return GPUCapabilities{}, "", false
	}
	for _, e := range entries {
		if ctx.Err() != nil {
			return GPUCapabilities{}, "", false
		}
		name := e.Name()
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}
		vendorID, readErr := os.ReadFile(filepath.Join("/sys/class/drm", name, "device", "vendor"))
		if readErr != nil {
			continue
		}
		vid := strings.TrimSpace(string(vendorID))
		vendor := gpuVendorFromPCIID(vid)
		if vendor == GPUUnknown {
			continue
		}
		return GPUCapabilities{Vendor: vendor, Renderer: fmt.Sprintf("DRM device %s (%s, vendor %s, no GL renderer)", name, vendor, vid)}, strings.ToLower(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(vid)), "0x")), true
	}
	return GPUCapabilities{}, "", false
}
