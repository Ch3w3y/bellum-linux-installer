package core

import (
	"strings"
)

// GPUVendor is the recognized GPU manufacturer. Unknown is deliberately the
// zero value so failed or incomplete identification grants no capabilities.
type GPUVendor string

const (
	GPUUnknown GPUVendor = "Unknown"
	GPUNVIDIA  GPUVendor = "NVIDIA"
	GPUAMD     GPUVendor = "AMD"
	GPUIntel   GPUVendor = "Intel"
)

// GPUCapabilities describes features the installer can safely configure.
// Detection is based on the active OpenGL renderer; hybrid systems may expose
// only one adapter; Ambiguous is set only when the renderer explicitly includes
// a PRIME/hybrid/mux marker. Absence of this flag does not prove there is one GPU.
type GPUCapabilities struct {
	Vendor          GPUVendor
	Renderer        string
	Generation      string
	Ambiguous       bool
	DLSS            bool
	NVAPI           bool
	FSR             bool
	FSR41           bool
	FrameGeneration bool
}

// DetectGPUCapabilitiesWith makes renderer acquisition injectable for callers
// and tests. Empty/unrecognized renderers remain unknown with no enabled flags.
func DetectGPUCapabilitiesWith(readRenderer func() (string, error)) (GPUCapabilities, error) {
	renderer, err := readRenderer()
	if err != nil {
		return GPUCapabilities{}, err
	}
	return ClassifyGPUCapabilities(renderer), nil
}

func DetectGPUCapabilities() (GPUCapabilities, error) {
	return DetectGPUCapabilitiesWith(func() (string, error) {
		output, err := RunCommandWithOutput([]string{"glxinfo", "-B"})
		if err != nil {
			return "", err
		}
		return parseRenderer(output), nil
	})
}

// DetectGPU preserves the historical vendor-string API for cleanup code.
func DetectGPU() (string, error) {
	caps, err := DetectGPUCapabilities()
	if err != nil {
		return "", err
	}
	if caps.Vendor == GPUUnknown {
		return "", nil
	}
	return string(caps.Vendor), nil
}

func parseRenderer(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(strings.ToLower(line), "opengl renderer") {
			if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// ClassifyGPUCapabilities uses explicit renderer identifiers only. Generation
// and optional features are enabled only where the renderer provides evidence.
func ClassifyGPUCapabilities(renderer string) GPUCapabilities {
	r := strings.ToLower(renderer)
	c := GPUCapabilities{Vendor: GPUUnknown, Renderer: renderer}
	if strings.Contains(r, "nvidia") || strings.Contains(r, "geforce") || strings.Contains(r, "quadro") {
		c.Vendor = GPUNVIDIA
		// RTX names encode architecture; GTX 16xx is Turing, GTX 10xx is Pascal.
		if strings.Contains(r, "rtx 50") {
			c.Generation = "Blackwell"
			c.DLSS, c.NVAPI, c.FrameGeneration = true, true, true
		} else if strings.Contains(r, "rtx 40") {
			c.Generation = "Ada"
			c.DLSS, c.NVAPI, c.FrameGeneration = true, true, true
		} else if strings.Contains(r, "rtx 30") {
			c.Generation = "Ampere"
			c.DLSS, c.NVAPI = true, true
		} else if strings.Contains(r, "rtx 20") || strings.Contains(r, "gtx 16") {
			c.Generation = "Turing"
			c.DLSS, c.NVAPI = true, true
		}
		c.FSR = true
	} else if strings.Contains(r, "amd") || strings.Contains(r, "radeon") || strings.Contains(r, "advanced micro devices") {
		c.Vendor = GPUAMD
		if strings.Contains(r, "gfx12") || strings.Contains(r, "rx 90") {
			c.Generation = "RDNA4"
			c.FSR, c.FSR41, c.FrameGeneration = true, true, true
		} else if strings.Contains(r, "gfx11") || strings.Contains(r, "rx 7") {
			c.Generation = "RDNA3"
			c.FSR = true
		} else if strings.Contains(r, "gfx10") || strings.Contains(r, "rx 6") {
			c.Generation = "RDNA2"
			c.FSR = true
		} else if strings.Contains(r, "gfx9") {
			c.Generation = "RDNA1/CDNA"
			c.FSR = true
		} else {
			c.FSR = true
		}
	} else if strings.Contains(r, "intel") || strings.Contains(r, "arc ") {
		c.Vendor = GPUIntel
		c.FSR = true
	}
	// A renderer string cannot reliably identify a second, inactive adapter.
	// Report explicit hybrid/PRIME markers as ambiguity and avoid architecture claims;
	// ordinary glxinfo output cannot reveal an inactive adapter.
	if strings.Contains(r, "prime") || strings.Contains(r, "hybrid") || strings.Contains(r, "mux") {
		c.Ambiguous = true
		c.Generation = ""
		c.DLSS, c.NVAPI, c.FSR41, c.FrameGeneration = false, false, false, false
	}
	return c
}

func classifyGPU(renderer string) string {
	caps := ClassifyGPUCapabilities(renderer)
	if caps.Vendor == GPUUnknown {
		parts := strings.Fields(renderer)
		if len(parts) > 0 {
			return parts[0]
		}
		return ""
	}
	return string(caps.Vendor)
}
