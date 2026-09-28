package core

import (
	"context"
	"testing"
)

func steamRootFS(home string) *fixtureFS {
	fsys := newFixtureFS()
	fsys.addDir(home + "/.local/share/Steam/steamapps")
	fsys.addDir(home + "/.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps")
	fsys.addDir(home + "/snap/steam/common/.local/share/Steam/steamapps")
	return fsys
}

func TestSteamAllKinds(t *testing.T) {
	home := "/home/tester"
	info, _ := DetectSteam(context.Background(), steamRootFS(home), home, 1000, true)
	kinds := map[SteamInstallKind]bool{}
	for _, in := range info.Installs {
		kinds[in.Kind] = true
	}
	if !kinds[SteamNative] || !kinds[SteamFlatpak] || !kinds[SteamSnap] {
		t.Fatalf("all kinds: %+v", info)
	}
	if len(info.Installs) != 3 || info.Discovery != DiscoveryComplete {
		t.Fatalf("count/discovery: %+v", info)
	}
}

func TestSteamSymlinkAliasesDeduplicated(t *testing.T) {
	home := "/home/tester"
	fsys := steamRootFS(home)
	fsys.addLink(home+"/.steam/steam", home+"/.local/share/Steam")
	info, _ := DetectSteam(context.Background(), fsys, home, 1000, true)
	native := 0
	for _, in := range info.Installs {
		if in.Kind == SteamNative {
			native++
		}
	}
	if native != 1 {
		t.Fatalf("aliases must dedupe: %+v", info.Installs)
	}
}

func TestSteamCyclicAndEscapingLinks(t *testing.T) {
	home := "/home/tester"
	fsys := steamRootFS(home)
	fsys.addLink(home+"/.steam/root", home+"/.steam/loop")
	fsys.addLink(home+"/.steam/loop", home+"/.steam/root")
	fsys.addLink(home+"/.steam/steam", "HOST:/etc/shadow")
	info, diags := DetectSteam(context.Background(), fsys, home, 1000, true)
	if info.Discovery != DiscoveryPartial {
		t.Fatalf("links must mark partial: %+v diags=%v", info, diags)
	}
	for _, in := range info.Installs {
		if in.Root == "/etc/shadow" {
			t.Fatal("escaping link must never resolve to host content")
		}
	}
}

func TestSteamEmptyParentInsufficient(t *testing.T) {
	home := "/home/tester"
	fsys := newFixtureFS()
	fsys.addDir(home + "/.local/share/Steam")
	info, _ := DetectSteam(context.Background(), fsys, home, 1000, true)
	if len(info.Installs) != 0 {
		t.Fatalf("empty parent: %+v", info)
	}
}

func TestSteamConfigProofWithoutSteamapps(t *testing.T) {
	home := "/home/tester"
	fsys := newFixtureFS()
	fsys.addFile(home+"/.local/share/Steam/config/loginusers.vdf", []byte(`"users" { "1" { "AccountName" "t" } }`))
	info, _ := DetectSteam(context.Background(), fsys, home, 1000, true)
	if len(info.Installs) != 1 {
		t.Fatalf("config proof: %+v", info)
	}
}

func TestSteamNoHomeIsUnknown(t *testing.T) {
	info, diags := DetectSteam(context.Background(), newFixtureFS(), "", 1000, true)
	if info.Discovery != DiscoveryUnknown || len(diags) == 0 {
		t.Fatalf("no home: %+v %v", info, diags)
	}
}

func TestSteamRunningStates(t *testing.T) {
	mkProc := func() *fixtureFS {
		fsys := newFixtureFS()
		fsys.addFile("/proc/1/comm", []byte("systemd\n"))
		fsys.addFile("/proc/1/status", []byte("Name:\tsystemd\nUid:\t0\t0\t0\t0\n"))
		fsys.addFile("/proc/4242/comm", []byte("steam\n"))
		fsys.addFile("/proc/4242/status", []byte("Name:\tsteam\nUid:\t1000\t1000\t1000\t1000\n"))
		return fsys
	}
	info, _ := DetectSteam(context.Background(), mkProc(), "/home/tester", 1000, true)
	if info.Running != TriYes {
		t.Fatalf("same-user steam: %+v", info)
	}
	// Different user does not count.
	info, _ = DetectSteam(context.Background(), mkProc(), "/home/tester", 1001, true)
	if info.Running != TriNo {
		t.Fatalf("other-user steam: %+v", info)
	}
	// Unreadable ownership means unknown.
	fsys := newFixtureFS()
	fsys.addFile("/proc/4242/comm", []byte("steam\n"))
	fsys.addDenied("/proc/4242/status")
	info, _ = DetectSteam(context.Background(), fsys, "/home/tester", 1000, true)
	if info.Running != TriUnknown {
		t.Fatalf("ownership unknown: %+v", info)
	}
	// No steam at all.
	fsys = newFixtureFS()
	fsys.addFile("/proc/1/comm", []byte("systemd\n"))
	info, _ = DetectSteam(context.Background(), fsys, "/home/tester", 1000, true)
	if info.Running != TriNo {
		t.Fatalf("stopped: %+v", info)
	}
	// Unreadable /proc means unknown.
	fsys = newFixtureFS()
	fsys.addDenied("/proc")
	info, _ = DetectSteam(context.Background(), fsys, "/home/tester", 1000, true)
	if info.Running != TriUnknown {
		t.Fatalf("proc denied: %+v", info)
	}
}
