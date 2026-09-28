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
		{"NVIDIA GeForce RTX 3080", GPUNVIDIA, "Ampere", true, true, true, false, false},
		{"NVIDIA GeForce RTX 4090", GPUNVIDIA, "Ada", true, true, true, false, true},
		{"NVIDIA GeForce RTX 5080", GPUNVIDIA, "Blackwell", true, true, true, false, true},
		{"AMD Radeon RX 7900 XTX gfx1100", GPUAMD, "RDNA3", false, false, true, false, false},
		{"AMD Radeon RX 9070 XT gfx1201", GPUAMD, "RDNA4", false, false, true, true, true},
		{"Intel Arc A770", GPUIntel, "", false, false, true, false, false},
		{"llvmpipe (LLVM 18.1.2)", GPUUnknown, "", false, false, false, false, false},
		{"NVIDIA GeForce RTX 4090 PRIME hybrid", GPUNVIDIA, "", false, false, true, false, false},

		// GTX 16xx is Turing but has no tensor cores, so no DLSS.
		{"NVIDIA GeForce GTX 1660 SUPER", GPUNVIDIA, "Turing", false, true, true, false, false},
		// Workstation Ada parts reuse GeForce RTX 40/50 numbering.
		{"NVIDIA RTX 5000 Ada Generation/PCIe/SSE2", GPUNVIDIA, "Ada", true, true, true, false, true},
		// gfx103x is RDNA2, including the Steam Deck; gfx101x is RDNA1. The old
		// gfx10 prefix match reported both as RDNA2.
		{"AMD Radeon Graphics (radeonsi, gfx1033, ACO, DRM 3.57)", GPUAMD, "RDNA2", false, false, true, false, false},
		{"AMD Radeon RX 6800 XT (radeonsi, gfx1030, ACO)", GPUAMD, "RDNA2", false, false, true, false, false},
		{"AMD Radeon RX 5700 XT (radeonsi, gfx1010, ACO)", GPUAMD, "RDNA1", false, false, true, false, false},
		// gfx9 is Vega and CDNA, not RDNA1 as it was previously labelled.
		{"AMD Radeon RX Vega (radeonsi, gfx900, ACO)", GPUAMD, "GCN/CDNA", false, false, true, false, false},
		// Polaris and an unmapped gfx10 stay generation-less rather than guess.
		{"AMD Radeon RX 580 (radeonsi, polaris10, ACO)", GPUAMD, "", false, false, true, false, false},
		{"AMD Radeon Graphics (radeonsi, gfx1040, ACO)", GPUAMD, "", false, false, true, false, false},
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

// lspci -nn fixtures: the class and vendor IDs are what the parser reads, so
// the samples keep the real `[class]` and `[vendor:device]` brackets.
const (
	lspciNVIDIA4090 = `01:00.0 VGA compatible controller [0300]: NVIDIA Corporation AD102 [GeForce RTX 4090] [10de:2684] (rev a1)`
	lspciAMD7900XTX = `03:00.0 VGA compatible controller [0300]: Advanced Micro Devices, Inc. [AMD/ATI] Navi 31 [Radeon RX 7900 XTX] [1002:744c] (rev cc)`
	lspciAMD9070XT  = `03:00.0 VGA compatible controller [0300]: Advanced Micro Devices, Inc. [AMD/ATI] Navi 48 [Radeon RX 9070 XT] [1002:7550] (rev c0)`
	lspciIntelUHD   = `00:02.0 Display controller [0380]: Intel Corporation Alder Lake-P GT2 [UHD Graphics] [8086:46a6] (rev 0c)`
	lspciHostBridge = `00:00.0 Host bridge [0600]: Intel Corporation 12th Gen Core Processor Host Bridge [8086:4648] (rev 02)`
	lspciNVMe       = `02:00.0 Non-Volatile memory controller [0108]: Samsung Electronics Co Ltd NVMe SSD Controller [144d:a80a] (rev 01)`
)

func TestReadRendererFromLspciOutputPicksFirstGPU(t *testing.T) {
	output := strings.Join([]string{lspciHostBridge, lspciNVIDIA4090, lspciNVMe}, "\n")
	renderer, ok := readRendererFromLspciOutput(output)
	if !ok {
		t.Fatal("expected lspci output to yield a renderer")
	}
	if got := ClassifyGPUCapabilities(renderer).Vendor; got != GPUNVIDIA {
		t.Fatalf("expected NVIDIA classification, got %q (renderer %q)", got, renderer)
	}
}

func TestParseLspciRendererLineVendors(t *testing.T) {
	if got, ok := parseLspciRendererLine(lspciAMD7900XTX); !ok || !strings.Contains(got, "AMD") {
		t.Fatalf("AMD line: got %q ok=%v", got, ok)
	}
	if got, ok := parseLspciRendererLine(lspciIntelUHD); !ok || !strings.Contains(got, "Intel") {
		t.Fatalf("Intel line: got %q ok=%v", got, ok)
	}
	if got, ok := parseLspciRendererLine(lspciNVIDIA4090); !ok || !strings.Contains(got, "NVIDIA") {
		t.Fatalf("NVIDIA line: got %q ok=%v", got, ok)
	}
	if _, ok := parseLspciRendererLine(lspciNVMe); ok {
		t.Fatal("NVMe device must not classify as GPU")
	}
}

// The vendor comes from the PCI ID, so a GPU-class device from a vendor the
// installer has no settings for is rejected rather than half-identified.
func TestParseLspciRendererLineRejectsUnknownVendorID(t *testing.T) {
	virtio := `00:02.0 VGA compatible controller [0300]: Red Hat, Inc. Virtio GPU [1af4:1050] (rev 01)`
	if got, ok := parseLspciRendererLine(virtio); ok {
		t.Fatalf("unknown PCI vendor must not classify as a supported GPU, got %q", got)
	}
}

// An ATI-branded card still carries vendor ID 1002, which name matching on
// "amd" alone would miss.
func TestParseLspciRendererLineMatchesATIByVendorID(t *testing.T) {
	line := `01:00.0 VGA compatible controller [0300]: ATI Technologies Inc Radeon HD 5770 [1002:68b8]`
	got, ok := parseLspciRendererLine(line)
	if !ok || !strings.HasPrefix(got, "AMD ") {
		t.Fatalf("ATI line must classify as AMD, got %q ok=%v", got, ok)
	}
}

func TestParseLspciRendererLineLocalizedClassLabels(t *testing.T) {
	line := `01:00.0 Affichage [0300]: NVIDIA Corporation AD104 [GeForce RTX 4070] [10de:2786] (rev a1)`
	if got, ok := parseLspciRendererLine(line); !ok || !strings.Contains(got, "NVIDIA") {
		t.Fatalf("localized class label must still match on class code, got %q ok=%v", got, ok)
	}
}

// The RX 9070 XT lspci -nn line must survive the vendor-ID parse and still
// classify as AMD RDNA4 with the FSR4 policy (the v2.2.0 hardware result).
func TestLspciRX9070XTClassifiesRDNA4(t *testing.T) {
	renderer, ok := readRendererFromLspciOutput(lspciAMD9070XT)
	if !ok {
		t.Fatal("expected the RX 9070 XT lspci line to yield a renderer")
	}
	caps := ClassifyGPUCapabilities(renderer)
	if caps.Vendor != GPUAMD || caps.Generation != "RDNA4" {
		t.Fatalf("expected AMD RDNA4, got %+v (renderer %q)", caps, renderer)
	}
	if !caps.FSR || !caps.FSR41 || !caps.FrameGeneration {
		t.Fatalf("expected the RDNA4 FSR4 policy, got %+v", caps)
	}
}

func TestReadRendererFromLspciNoGPU(t *testing.T) {
	output := strings.Join([]string{
		lspciHostBridge,
		`00:14.3 Network controller [0280]: Intel Corporation Alder Lake-P PCH CNVi WiFi [8086:51f0] (rev 01)`,
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
		func() (string, bool) { return readRendererFromLspciOutput(lspciNVIDIA4090) },
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
			return GPUCapabilities{Vendor: GPUAMD, Renderer: "DRM device card0 (AMD, vendor 0x1002, no GL renderer)"}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(renderer, "card0") || !strings.Contains(renderer, "0x1002") {
		t.Fatalf("expected DRM renderer string, got %q", renderer)
	}
	if got := ClassifyGPUCapabilities(renderer).Vendor; got != GPUAMD {
		t.Fatalf("DRM fallback renderer must preserve mapped vendor, got %s (%q)", got, renderer)
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

func TestReadRendererFallsBackWhenGlxinfoCannotOpenDisplay(t *testing.T) {
	renderer, err := readRendererWithFallbackWith(
		func() (string, error) { return "", fmt.Errorf("glxinfo exited with status 1: unable to open display") },
		func() (string, bool) { return readRendererFromLspciOutput(lspciNVIDIA4090) },
		func() (GPUCapabilities, error) {
			t.Fatal("DRM fallback must not run when lspci succeeds")
			return GPUCapabilities{}, fmt.Errorf("unused")
		},
	)
	if err != nil || !strings.Contains(renderer, "4090") {
		t.Fatalf("expected lspci renderer, got %q, %v", renderer, err)
	}
}

// glxinfo can exit 0 while printing no renderer line at all (software GL, a
// stubbed driver). An empty renderer must not short-circuit the chain.
func TestReadRendererWithFallbackRejectsEmptyGlxinfoRenderer(t *testing.T) {
	renderer, err := readRendererWithFallbackWith(
		func() (string, error) { return "   \n", nil },
		func() (string, bool) { return readRendererFromLspciOutput(lspciAMD7900XTX) },
		func() (GPUCapabilities, error) {
			t.Fatal("DRM fallback must not run when lspci succeeds")
			return GPUCapabilities{}, fmt.Errorf("unused")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if caps := ClassifyGPUCapabilities(renderer); caps.Vendor != GPUAMD || caps.Generation != "RDNA3" {
		t.Fatalf("expected AMD RDNA3 from the lspci fallback, got %+v", caps)
	}
}

func TestReadRendererWithFallbackReportsEmptyRendererWhenNothingElseFound(t *testing.T) {
	_, err := readRendererWithFallbackWith(
		func() (string, error) { return "", nil },
		func() (string, bool) { return "", false },
		func() (GPUCapabilities, error) {
			return GPUCapabilities{}, fmt.Errorf("no supported DRM GPU device found")
		},
	)
	if err == nil {
		t.Fatal("an empty renderer with no fallback must be an error, not a silent unknown GPU")
	}
	if !strings.Contains(err.Error(), "OpenGL renderer") {
		t.Fatalf("expected the empty-renderer reason to surface, got %v", err)
	}
}

// The DRM probe's renderer string is handed back to ClassifyGPUCapabilities, so
// it has to carry the vendor name it detected.
func TestDRMRendererStringClassifiesToTheDetectedVendor(t *testing.T) {
	for vendor, id := range map[GPUVendor]string{GPUAMD: "0x1002", GPUNVIDIA: "0x10de", GPUIntel: "0x8086"} {
		renderer := fmt.Sprintf("DRM device card0 (%s, vendor %s, no GL renderer)", vendor, id)
		if got := ClassifyGPUCapabilities(renderer).Vendor; got != vendor {
			t.Fatalf("DRM renderer %q classified as %q, want %q", renderer, got, vendor)
		}
	}
}

func TestGPUVendorFromPCIID(t *testing.T) {
	for id, want := range map[string]GPUVendor{
		"0x10de": GPUNVIDIA, "10DE": GPUNVIDIA,
		"0x1002": GPUAMD, "1022": GPUAMD,
		"0x8086": GPUIntel,
		"1af4":   GPUUnknown, "": GPUUnknown, "0x": GPUUnknown,
	} {
		if got := gpuVendorFromPCIID(id); got != want {
			t.Fatalf("gpuVendorFromPCIID(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestDetectGPUCapabilitiesFromDRMMatchesKnownVendors(t *testing.T) {
	renderer, err := readRendererWithFallbackWith(
		func() (string, error) {
			return "", &exec.Error{Name: "glxinfo", Err: exec.ErrNotFound}
		},
		func() (string, bool) { return "", false },
		func() (GPUCapabilities, error) {
			return GPUCapabilities{Vendor: GPUAMD, Renderer: "DRM device card0 (AMD, vendor 0x1002, no GL renderer)"}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(renderer, "card0") {
		t.Fatalf("fallback renderer must identify the DRM card, got %q", renderer)
	}
}

// Review follow-ups for the whole-token patterns: Mesa revision suffixes,
// laptop marketing names, pre-24.1 Mesa codenames and Turing Quadro RTX.
func TestClassifyGPURevisionSuffixesCodenamesAndWorkstations(t *testing.T) {
	for renderer, want := range map[string]string{
		"AMD Radeon 780M (radeonsi, gfx1103_r1, LLVM 18.1.8, DRM 3.57)": "RDNA3",
		"AMD Radeon 760M (radeonsi, gfx1103_r2, ACO, DRM 3.59)":         "RDNA3",
		"AMD Radeon RX 7600S (radeonsi, navi33, LLVM 15.0.7, DRM 3.49)": "RDNA3",
		"AMD Radeon RX 6800M (navi22, LLVM 15.0.7, DRM 3.49)":           "RDNA2",
		"AMD Radeon RX 6800 XT (navi21, LLVM 15.0.7, DRM 3.49)":         "RDNA2",
		"AMD Custom GPU 0405 (vangogh, LLVM 15.0.7, DRM 3.49)":          "RDNA2",
		"AMD Radeon Graphics (rembrandt, LLVM 15.0.7, DRM 3.49)":        "RDNA2",
		"AMD Radeon RX 5700 XT (navi10, LLVM 15.0.7, DRM 3.49)":         "RDNA1",
		"AMD Radeon RX 7900 XTX (navi31, LLVM 16.0.6, DRM 3.54)":        "RDNA3",
		"AMD Radeon 890M (radeonsi, strix_halo, ACO)":                   "RDNA3",
		"AMD Radeon RX 9070 XT (radeonsi, gfx1201_r1, ACO)":             "RDNA4",
		"AMD Radeon RX 580 Series (radeonsi, polaris10, ACO, DRM 3.57)": "",
		"AMD Radeon Graphics (radeonsi, gfx11000, ACO)":                 "",
		"Quadro RTX 5000/PCIe/SSE2":                                     "Turing",
		"Quadro RTX 4000/PCIe/SSE2":                                     "Turing",
		"NVIDIA RTX 5000 Ada Generation/PCIe/SSE2":                      "Ada",
		"NVIDIA GeForce RTX 5070/PCIe/SSE2":                             "Blackwell",
	} {
		if got := ClassifyGPUCapabilities(renderer).Generation; got != want {
			t.Errorf("%q: generation %q, want %q", renderer, got, want)
		}
	}
	if c := ClassifyGPUCapabilities("Quadro RTX 5000/PCIe/SSE2"); c.FrameGeneration || !c.DLSS {
		t.Errorf("Turing Quadro RTX: %+v", c)
	}
}
