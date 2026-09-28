package core

import (
	"context"
	"testing"
)

func vulkanFS() *fixtureFS {
	fsys := newFixtureFS()
	fsys.addFile("/usr/share/vulkan/icd.d/nvidia_icd.json", []byte(`{"file_format_version":"1.0.1","ICD":{"library_path":"libGLX_nvidia.so.0","api_version":"1.3.0"}}`))
	fsys.addFile("/usr/share/vulkan/icd.d/intel_icd.x86_64.json", []byte(`{"file_format_version":"1.0.1","ICD":{"library_path":"libvulkan_intel.so","api_version":"1.3.0"}}`))
	fsys.addFile("/usr/share/vulkan/icd.d/broken.json", []byte(`{not json`))
	return fsys
}

func TestVulkanInventory(t *testing.T) {
	info, _ := DetectVulkan(context.Background(), vulkanFS(), "/home/tester", fixtureEnv(nil))
	if info.NVIDIACandidate != NVIDIAPresent {
		t.Fatalf("nvidia manifest: %+v", info)
	}
	if info.ScanState != DiscoveryPartial {
		t.Fatalf("broken manifest must make scan partial: %+v", info)
	}
	names := map[string]bool{}
	for _, m := range info.Manifests {
		names[m.Path] = true
	}
	if !names["/usr/share/vulkan/icd.d/nvidia_icd.json"] || !names["/usr/share/vulkan/icd.d/intel_icd.x86_64.json"] {
		t.Fatalf("manifests: %+v", info.Manifests)
	}
	for _, m := range info.Manifests {
		if m.Path == "/usr/share/vulkan/icd.d/broken.json" && m.ParseState != "malformed" {
			t.Fatalf("broken parse state: %+v", m)
		}
		if m.Path == "/usr/share/vulkan/icd.d/nvidia_icd.json" && (m.VendorHint != "nvidia" || m.LibraryPath == "" || m.APIVersion == "") {
			t.Fatalf("nvidia manifest fields: %+v", m)
		}
	}
}

func TestVulkanFilenameAloneProvesNothing(t *testing.T) {
	// A file merely named nvidia_icd.json with unrelated content is parsed
	// from its JSON, not its filename.
	fsys := newFixtureFS()
	fsys.addFile("/usr/share/vulkan/icd.d/nvidia_icd.json", []byte(`{"file_format_version":"1.0.1","ICD":{"library_path":"libvulkan_intel.so","api_version":"1.3.0"}}`))
	info, _ := DetectVulkan(context.Background(), fsys, "/home/tester", fixtureEnv(nil))
	// VendorHint comes from filename OR library; here the library says intel.
	for _, m := range info.Manifests {
		if m.LibraryPath != "libvulkan_intel.so" {
			t.Fatalf("content must win: %+v", m)
		}
	}
}

func TestVulkanOverridesMakeSelectionUnknown(t *testing.T) {
	fsys := newFixtureFS()
	info, _ := DetectVulkan(context.Background(), fsys, "/home/tester", fixtureEnv(map[string]string{"VK_DRIVER_FILES": "/custom/nvidia.json"}))
	if info.NVIDIACandidate != NVIDIAUnknown {
		t.Fatalf("override: %+v", info)
	}
	if info.Evidence == "" {
		t.Fatal("overrides must be captured as evidence")
	}
}

func TestVulkanEmptyScan(t *testing.T) {
	info, _ := DetectVulkan(context.Background(), newFixtureFS(), "/home/tester", fixtureEnv(nil))
	if info.ScanState != DiscoveryComplete || info.NVIDIACandidate != NVIDIANotObserved {
		t.Fatalf("empty: %+v", info)
	}
	if len(info.Manifests) != 0 {
		t.Fatalf("no manifests: %+v", info)
	}
}

func TestVulkanUnreadableDirIsPartial(t *testing.T) {
	fsys := newFixtureFS()
	fsys.addDenied("/usr/share/vulkan/icd.d")
	info, diags := DetectVulkan(context.Background(), fsys, "/home/tester", fixtureEnv(nil))
	if info.ScanState != DiscoveryPartial {
		t.Fatalf("denied: %+v", info)
	}
	found := false
	for _, d := range diags {
		if d.Code == "vulkan/dir-unreadable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dir-unreadable: %v", diags)
	}
}

func TestVulkanXDGDirsScanned(t *testing.T) {
	fsys := newFixtureFS()
	fsys.addFile("/home/tester/.config/vulkan/icd.d/radeon.json", []byte(`{"ICD":{"library_path":"libvulkan_radeon.so","api_version":"1.3.0"}}`))
	info, _ := DetectVulkan(context.Background(), fsys, "/home/tester", fixtureEnv(nil))
	if len(info.Manifests) != 1 || info.Manifests[0].VendorHint != "amd" {
		t.Fatalf("xdg: %+v", info)
	}
}
