package core

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestClassifyGPUCapabilities(t *testing.T) {
	tests := []struct {
		renderer                    string
		vendor                      GPUVendor
		generation                  string
		dlss, nvapi, fsr, fsr41, fg bool
	}{
		{"NVIDIA GeForce RTX 2080", GPUNVIDIA, "Turing", true, true, true, false, false},
		{"NVIDIA GeForce RTX 4090", GPUNVIDIA, "Ada", true, true, true, false, true},
		{"AMD Radeon RX 7900 XTX gfx1100", GPUAMD, "RDNA3", false, false, true, false, false},
		{"AMD Radeon RX 9070 XT gfx1201", GPUAMD, "RDNA4", false, false, true, true, true},
		{"Intel Arc A770", GPUIntel, "", false, false, true, false, false},
		{"llvmpipe (LLVM 18.1.2)", GPUUnknown, "", false, false, false, false, false},
		{"NVIDIA GeForce RTX 4090 PRIME hybrid", GPUNVIDIA, "", false, false, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.renderer, func(t *testing.T) {
			got := ClassifyGPUCapabilities(tt.renderer)
			if got.Vendor != tt.vendor || got.Generation != tt.generation || got.DLSS != tt.dlss || got.NVAPI != tt.nvapi || got.FSR != tt.fsr || got.FSR41 != tt.fsr41 || got.FrameGeneration != tt.fg {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestDetectGPUCapabilitiesWith(t *testing.T) {
	want := "AMD Radeon RX 9070 gfx1200"
	got, err := DetectGPUCapabilitiesWith(func() (string, error) { return want, nil })
	if err != nil || got.Renderer != want || !got.FSR41 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestGLXInfoMissingError(t *testing.T) {
	if !glxinfoMissingError(&exec.Error{Name: "glxinfo", Err: exec.ErrNotFound}) {
		t.Fatal("expected exec.Error not-found to be recognized as missing")
	}
	if !glxinfoMissingError(fmt.Errorf("exec: \"glxinfo\": executable file not found in $PATH")) {
		t.Fatal("expected wrapped exec error text to be recognized as missing")
	}
	if glxinfoMissingError(nil) {
		t.Fatal("nil error must not count as missing")
	}
	if glxinfoMissingError(fmt.Errorf("exit status 1")) {
		t.Fatal("non-lookup failures must not count as missing")
	}
}

func TestReadRendererFromLspciOutputPicksFirstGPU(t *testing.T) {
	output := strings.Join([]string{
		`00:00.0 "Host bridge [0600]" "Intel" "12th Gen Core Processor"`,
		`01:00.0 "VGA compatible controller [0300]" "NVIDIA" "AD103 [GeForce RTX 4090]"`,
		`02:00.0 "Non-Volatile memory controller [0108]" "Samsung" "NVMe SSD"`,
	}, "\n")
	renderer, ok := readRendererFromLspciOutput(output)
	if !ok {
		t.Fatal("expected lspci output to yield a renderer")
	}
	if got := ClassifyGPUCapabilities(renderer).Vendor; got != GPUNVIDIA {
		t.Fatalf("expected NVIDIA classification, got %q (renderer %q)", got, renderer)
	}
}

func TestParseLspciRendererLineVendors(t *testing.T) {
	if got, ok := parseLspciRendererLine(`01:00.0 "VGA compatible controller [0300]" "Advanced Micro Devices, Inc. [AMD/ATI]" "Navi 31 [Radeon RX 7900 XTX]"`); !ok || !strings.Contains(got, "AMD") {
		t.Fatalf("AMD line: got %q ok=%v", got, ok)
	}
	if got, ok := parseLspciRendererLine(`00:02.0 "Display controller [0380]" "Intel" "Alder Lake-P GT2 [UHD Graphics]"`); !ok || !strings.Contains(got, "Intel") {
		t.Fatalf("Intel line: got %q ok=%v", got, ok)
	}
	if got, ok := parseLspciRendererLine(`01:00.0 "VGA compatible controller [0300]" "NVIDIA" "AD103 [GeForce RTX 4090]"`); !ok || !strings.Contains(got, "NVIDIA") {
		t.Fatalf("NVIDIA line: got %q ok=%v", got, ok)
	}
	if _, ok := parseLspciRendererLine(`03:00.0 "Non-Volatile memory controller [0108]" "Samsung" "NVMe SSD"`); ok {
		t.Fatal("NVMe device must not classify as GPU")
	}
}

func TestParseLspciRendererLineLocalizedClassLabels(t *testing.T) {
	if got, ok := parseLspciRendererLine(`01:00.0 "Affichage [0300]" "NVIDIA" "AD104 [GeForce RTX 4070]"`); !ok || !strings.Contains(got, "NVIDIA") {
		t.Fatalf("localized class label must still match on class code, got %q ok=%v", got, ok)
	}
}

func TestReadRendererFromLspciNoGPU(t *testing.T) {
	output := strings.Join([]string{
		`00:00.0 "Host bridge [0600]" "Intel" "12th Gen Core Processor"`,
		`00:14.3 "Network controller [0280]" "Intel" "Alder Lake-P PCH CNVi WiFi"`,
	}, "\n")
	if _, ok := readRendererFromLspciOutput(output); ok {
		t.Fatal("expected ok=false when no GPU-class device is present")
	}
}

func TestDetectGPUCapabilitiesWithMissingGlxinfoIsStrict(t *testing.T) {
	caps, err := DetectGPUCapabilitiesWith(func() (string, error) {
		return "", &exec.Error{Name: "glxinfo", Err: exec.ErrNotFound}
	})
	if err == nil {
		t.Fatal("expected error when glxinfo is missing")
	}
	if caps != (GPUCapabilities{}) {
		t.Fatalf("injectable path must stay strict; got %+v", caps)
	}
}

func TestReadRendererWithFallbackPrefersGlxinfo(t *testing.T) {
	renderer, err := readRendererWithFallbackWith(
		func() (string, error) { return "OpenGL renderer string: AMD Radeon RX 9070 XT gfx1201", nil },
		func() (string, bool) { t.Fatal("lspci fallback must not run when glxinfo succeeds"); return "", false },
		func() (GPUCapabilities, error) {
			t.Fatal("DRM fallback must not run when glxinfo succeeds")
			return GPUCapabilities{}, fmt.Errorf("unused")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(renderer, "Radeon") {
		t.Fatalf("expected glxinfo renderer, got %q", renderer)
	}
}

func TestReadRendererWithFallbackUsesLspciWhenGlxinfoMissing(t *testing.T) {
	renderer, err := readRendererWithFallbackWith(
		func() (string, error) {
			return "", &exec.Error{Name: "glxinfo", Err: exec.ErrNotFound}
		},
		func() (string, bool) {
			return readRendererFromLspciOutput(`01:00.0 "VGA compatible controller [0300]" "NVIDIA" "AD103 [GeForce RTX 4090]"`)
		},
		func() (GPUCapabilities, error) {
			t.Fatal("DRM fallback must not run when lspci succeeds")
			return GPUCapabilities{}, fmt.Errorf("unused")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	caps := ClassifyGPUCapabilities(renderer)
	if caps.Vendor != GPUNVIDIA || caps.Generation != "Ada" {
		t.Fatalf("expected NVIDIA Ada from lspci fallback, got %+v", caps)
	}
}

func TestReadRendererWithFallbackUsesDRMWhenOnlySysfsAvailable(t *testing.T) {
	renderer, err := readRendererWithFallbackWith(
		func() (string, error) {
			return "", fmt.Errorf("exec: \"glxinfo\": executable file not found in $PATH")
		},
		func() (string, bool) { return "", false },
		func() (GPUCapabilities, error) {
			return GPUCapabilities{Vendor: GPUAMD, Renderer: "DRM device card0 (vendor 0x1002, no GL renderer)"}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(renderer, "card0") || !strings.Contains(renderer, "0x1002") {
		t.Fatalf("expected DRM renderer string, got %q", renderer)
	}
}

func TestReadRendererWithFallbackErrorsWhenNothingFound(t *testing.T) {
	_, err := readRendererWithFallbackWith(
		func() (string, error) {
			return "", &exec.Error{Name: "glxinfo", Err: exec.ErrNotFound}
		},
		func() (string, bool) { return "", false },
		func() (GPUCapabilities, error) {
			return GPUCapabilities{}, fmt.Errorf("no supported DRM GPU device found")
		},
	)
	if err == nil {
		t.Fatal("expected the original glxinfo error when all fallbacks find nothing")
	}
	if !glxinfoMissingError(err) {
		t.Fatalf("expected original lookup error to surface, got %v", err)
	}
}

func TestReadRendererWithFallbackPropagatesRealGlxinfoFailures(t *testing.T) {
	boom := fmt.Errorf("glxinfo exited with status 1: X server not running")
	_, err := readRendererWithFallbackWith(
		func() (string, error) { return "", boom },
		func() (string, bool) {
			t.Fatal("lspci fallback must not run for non-lookup failures")
			return "", false
		},
		func() (GPUCapabilities, error) {
			t.Fatal("DRM fallback must not run for non-lookup failures")
			return GPUCapabilities{}, fmt.Errorf("unused")
		},
	)
	if err != boom {
		t.Fatalf("expected original error to propagate, got %v", err)
	}
}

func TestDetectGPUCapabilitiesFromDRMMatchesKnownVendors(t *testing.T) {
	renderer, err := readRendererWithFallbackWith(
		func() (string, error) {
			return "", &exec.Error{Name: "glxinfo", Err: exec.ErrNotFound}
		},
		func() (string, bool) { return "", false },
		func() (GPUCapabilities, error) {
			return GPUCapabilities{Vendor: GPUAMD, Renderer: "DRM device card0 (vendor 0x1002, no GL renderer)"}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(renderer, "card0") {
		t.Fatalf("fallback renderer must identify the DRM card, got %q", renderer)
	}
}
