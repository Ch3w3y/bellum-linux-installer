package core

import "testing"

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
