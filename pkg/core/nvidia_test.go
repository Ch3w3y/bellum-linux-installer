package core

import (
	"context"
	"testing"
)

const proprietaryBanner = "NVRM version: NVIDIA UNIX x86_64 Kernel Module  550.54.14  Thu Feb 22 01:44:30 UTC 2024\nGCC version: gcc version 13.2.1\n"
const openBanner = "NVRM version: NVIDIA UNIX Open Kernel Module  550.54.14  Thu Feb 22 01:44:30 UTC 2024\n"

func nvidiaFS(files map[string]string, dirs ...string) *fixtureFS {
	fsys := newFixtureFS()
	for p, c := range files {
		fsys.addFile(p, []byte(c))
	}
	for _, d := range dirs {
		fsys.addDir(d)
	}
	return fsys
}

func TestNVIDIAProprietary(t *testing.T) {
	fsys := nvidiaFS(map[string]string{
		nvidiaVersionPath: "550.54.14\n",
		nvidiaBannerPath:  proprietaryBanner,
		nvidiaModesetPath: "Y\n",
	}, nvidiaModulePath)
	st, _ := DetectNVIDIA(context.Background(), fsys, "NVIDIA GeForce RTX 4070", []string{"/usr/lib/libGLX_nvidia.so.550.54.14"})
	if st.Kernel != NVIDIAKernelProprietary || st.Userspace != NVIDIAUserspaceNVIDIA {
		t.Fatalf("flavor/userspace: %+v", st)
	}
	if st.Version != "550.54.14" || st.VersionSrc != nvidiaVersionPath {
		t.Fatalf("version: %+v", st)
	}
	if st.Modeset != TriYes || st.Association != TriYes {
		t.Fatalf("modeset/association: %+v", st)
	}
}

func TestNVIDIAOpenDistinctFromNouveau(t *testing.T) {
	fsys := nvidiaFS(map[string]string{
		nvidiaVersionPath: "550.54.14\n",
		nvidiaBannerPath:  openBanner,
		nvidiaModesetPath: "N\n",
	}, nvidiaModulePath)
	st, _ := DetectNVIDIA(context.Background(), fsys, "NVIDIA GeForce RTX 4070", nil)
	if st.Kernel != NVIDIAKernelOpen {
		t.Fatalf("open module must stay distinct: %+v", st)
	}
	if st.Userspace != NVIDIAUserspaceNVIDIA {
		t.Fatalf("open kernel module uses NVIDIA userspace: %+v", st)
	}
	if st.Modeset != TriNo {
		t.Fatalf("modeset: %+v", st)
	}
}

func TestNVIDIAMissingBannerLeavesFlavorUnknown(t *testing.T) {
	fsys := nvidiaFS(map[string]string{nvidiaVersionPath: "550.54.14\n"}, nvidiaModulePath)
	st, diags := DetectNVIDIA(context.Background(), fsys, "NVIDIA GeForce RTX 4070", nil)
	if st.Kernel != NVIDIAKernelUnknown {
		t.Fatalf("version alone must not set flavor: %+v", st)
	}
	found := false
	for _, d := range diags {
		if d.Code == "nvidia/flavor-unknown" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected flavor-unknown diagnostic: %v", diags)
	}
}

func TestNVIDIANouveauAndConflict(t *testing.T) {
	fsys := nvidiaFS(nil, nouveauModulePath)
	st, _ := DetectNVIDIA(context.Background(), fsys, "Mesa Nouveau NV168", nil)
	if st.Kernel != NVIDIAKernelNouveau || st.Userspace != NVIDIAUserspaceNouveau {
		t.Fatalf("nouveau: %+v", st)
	}
	fsys = nvidiaFS(map[string]string{nvidiaBannerPath: proprietaryBanner}, nvidiaModulePath, nouveauModulePath)
	st, diags := DetectNVIDIA(context.Background(), fsys, "NVIDIA GeForce RTX 4070", nil)
	if st.Kernel != NVIDIAKernelUnknown {
		t.Fatalf("mixed modules must be ambiguous: %+v", st)
	}
	found := false
	for _, d := range diags {
		if d.Code == "nvidia/modules-conflict" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected modules-conflict: %v", diags)
	}
}

func TestNVIDIANotObserved(t *testing.T) {
	st, _ := DetectNVIDIA(context.Background(), newFixtureFS(), "AMD Radeon RX 9070 XT gfx1201", nil)
	if st.Kernel != NVIDIAKernelNotObserved || st.Association != TriUnknown {
		t.Fatalf("absent: %+v", st)
	}
}

func TestNVIDIAUnassociatedHybrid(t *testing.T) {
	fsys := nvidiaFS(map[string]string{nvidiaBannerPath: proprietaryBanner}, nvidiaModulePath)
	st, _ := DetectNVIDIA(context.Background(), fsys, "AMD Radeon Graphics (radeonsi, gfx1033, ACO)", nil)
	// The loaded module elsewhere does not drive the selected renderer:
	// system-level NVIDIA userspace stands, association is no.
	if st.Association != TriNo {
		t.Fatalf("hybrid association: %+v", st)
	}
	if st.Userspace != NVIDIAUserspaceNVIDIA {
		t.Fatalf("loaded NVIDIA stack keeps NVIDIA userspace: %+v", st)
	}
}

func TestNVIDIAModesetValues(t *testing.T) {
	for raw, want := range map[string]TriState{"Y": TriYes, "1": TriYes, "N": TriNo, "0": TriNo} {
		fsys := nvidiaFS(map[string]string{nvidiaModesetPath: raw + "\n"})
		st, _ := DetectNVIDIA(context.Background(), fsys, "", nil)
		if st.Modeset != want {
			t.Errorf("modeset %q: got %q want %q", raw, st.Modeset, want)
		}
	}
	fsys := nvidiaFS(map[string]string{nvidiaModesetPath: "maybe\n"})
	st, diags := DetectNVIDIA(context.Background(), fsys, "", nil)
	if st.Modeset != TriUnknown || len(diags) == 0 {
		t.Fatalf("unrecognized modeset: %+v %v", st, diags)
	}
}

func TestParseDriverVersion(t *testing.T) {
	if v, ok := ParseDriverVersion("550.54.14"); !ok || v != "550.54.14" {
		t.Fatalf("numeric: %q %v", v, ok)
	}
	if _, ok := ParseDriverVersion("no version here"); ok {
		t.Fatal("unparseable must fail")
	}
	fsys := nvidiaFS(map[string]string{nvidiaVersionPath: "not-a-version\n"}, nvidiaModulePath)
	st, diags := DetectNVIDIA(context.Background(), fsys, "", nil)
	if st.Version != "" || st.RawVersion == "" {
		t.Fatalf("raw retained, parsed unknown: %+v", st)
	}
	found := false
	for _, d := range diags {
		if d.Code == "nvidia/version-unparseable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected version-unparseable: %v", diags)
	}
}

func TestNVIDIANVKMarker(t *testing.T) {
	st, _ := DetectNVIDIA(context.Background(), newFixtureFS(), "NVK NV168", []string{"/usr/lib/libvulkan_nvk.so"})
	if st.Userspace != NVIDIAUserspaceNVK {
		t.Fatalf("nvk: %+v", st)
	}
}
