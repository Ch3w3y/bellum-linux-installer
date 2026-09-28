package core

import (
	"context"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Typed Steam inventory. The catalog covers native, Flatpak and Snap roots;
// step 2 adds this detection while step 5 migrates the EAC search, so the
// existing EAC resolution is deliberately untouched. Simultaneous
// installations of several kinds are all reported; no profile choice is
// ever requested.

type steamCandidate struct {
	kind SteamInstallKind
	rel  string
}

func steamCandidates() []steamCandidate {
	return []steamCandidate{
		{SteamNative, ".local/share/Steam"},
		{SteamNative, ".steam/steam"},
		{SteamNative, ".steam/root"},
		{SteamFlatpak, ".var/app/com.valvesoftware.Steam/.local/share/Steam"},
		{SteamFlatpak, ".var/app/com.valvesoftware.Steam/data/Steam"},
		{SteamSnap, "snap/steam/common/.local/share/Steam"},
	}
}

// DetectSteam inventories Steam roots and takes a best-effort same-user
// /proc snapshot of whether Steam is running. Inaccessible process
// ownership or listing means unknown; the snapshot is informational and must
// be rechecked at action time before any opted-in write.
func DetectSteam(ctx context.Context, fs DetectionFS, home string, euid int, haveEUID bool) (SteamInfo, []DetectionDiagnostic) {
	info := SteamInfo{Discovery: DiscoveryComplete, Running: TriUnknown}
	var diags []DetectionDiagnostic
	add := func(code, source string, outcome DiagnosticOutcome, detail string) {
		diags = append(diags, DetectionDiagnostic{Code: code, Subsystem: "steam", Source: source, Outcome: outcome, Detail: SanitizeField(detail, MaxDetailLen)})
	}

	if strings.TrimSpace(home) == "" {
		info.Discovery = DiscoveryUnknown
		add("steam/no-home", "steam", OutcomeMissing, "no home directory; Steam inventory unknown")
	} else {
		seen := map[string]SteamInstallKind{}
		var order []string
		partial := false
		for _, c := range steamCandidates() {
			if err := ctx.Err(); err != nil {
				partial = true
				add("steam/cancelled", "steam", OutcomeTimeout, "inventory cancelled")
				break
			}
			abs := path.Join(home, c.rel)
			canonical, rerr := resolveLink(fs, abs)
			if rerr != nil {
				if !isMissing(rerr) {
					partial = true
					add("steam/root-unresolvable", abs, outcomeOf(rerr), "Steam root alias unresolvable")
				}
				continue
			}
			proven, perr := steamRootProven(fs, canonical)
			if perr != nil {
				partial = true
				add("steam/root-unreadable", canonical, outcomeOf(perr), "Steam root candidate unreadable")
				continue
			}
			if !proven {
				continue
			}
			if prev, dup := seen[canonical]; dup {
				_ = prev
				continue
			}
			seen[canonical] = c.kind
			order = append(order, canonical)
		}
		sort.Strings(order)
		kindOf := func(root string) SteamInstallKind { return seen[root] }
		for _, root := range order {
			info.Installs = append(info.Installs, SteamInstall{Kind: kindOf(root), Root: root, Evidence: "root:" + string(kindOf(root))})
		}
		if partial {
			info.Discovery = DiscoveryPartial
		} else if !homeListable(fs, home) {
			info.Discovery = DiscoveryUnknown
			add("steam/home-unreadable", home, OutcomeMissing, "home directory unreadable; Steam inventory unknown")
		}
	}

	running, rdiags := detectSteamRunning(ctx, fs, euid, haveEUID)
	info.Running = running
	diags = append(diags, rdiags...)
	return info, diags
}

// steamRootProven counts a candidate only when its root holds a steamapps
// directory or readable Steam configuration. An empty parent is insufficient.
func steamRootProven(fs DetectionFS, root string) (bool, error) {
	if ok, err := isDir(fs, path.Join(root, "steamapps")); err == nil && ok {
		return true, nil
	} else if err != nil && !isMissing(err) && !isNotDir(err) {
		return false, err
	}
	for _, cfg := range []string{path.Join(root, "config", "loginusers.vdf"), path.Join(root, "config", "libraryfolders.vdf")} {
		if raw, err := readBounded(fs, cfg, ReadLimitText); err == nil && len(raw) > 0 {
			return true, nil
		} else if err != nil && !isMissing(err) {
			return false, err
		}
	}
	return false, nil
}

func homeListable(fs DetectionFS, home string) bool {
	_, err := fs.ReadDir(home)
	return err == nil
}

// detectSteamRunning matches same-user /proc process-name evidence. Only the
// name and UID facts are captured; command lines are never stored.
func detectSteamRunning(ctx context.Context, fs DetectionFS, euid int, haveEUID bool) (TriState, []DetectionDiagnostic) {
	var diags []DetectionDiagnostic
	names, err := readDirNames(fs, "/proc")
	if err != nil {
		diags = append(diags, DetectionDiagnostic{Code: "steam/proc-unreadable", Subsystem: "steam", Source: "/proc", Outcome: outcomeOf(err), Detail: "process listing unavailable; Steam running state unknown"})
		return TriUnknown, diags
	}
	checked := 0
	uidUnknown := false
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			diags = append(diags, DetectionDiagnostic{Code: "steam/proc-cancelled", Subsystem: "steam", Source: "/proc", Outcome: OutcomeTimeout, Detail: "process scan cancelled"})
			return TriUnknown, diags
		}
		if !isPID(name) {
			continue
		}
		checked++
		if checked > MaxScanEntries {
			diags = append(diags, DetectionDiagnostic{Code: "steam/proc-truncated", Subsystem: "steam", Source: "/proc", Outcome: OutcomeUnsupported, Detail: "process scan hit the entry bound"})
			return TriUnknown, diags
		}
		comm, cerr := readBounded(fs, path.Join("/proc", name, "comm"), ReadLimitSmall)
		if cerr != nil {
			if !isMissing(cerr) {
				uidUnknown = true
			}
			continue
		}
		if strings.TrimSpace(string(comm)) != "steam" {
			continue
		}
		if !haveEUID {
			uidUnknown = true
			continue
		}
		uid, uerr := procUID(fs, name)
		if uerr != nil {
			uidUnknown = true
			continue
		}
		if uid == euid {
			return TriYes, diags
		}
	}
	if checked == 0 {
		return TriUnknown, diags
	}
	if uidUnknown {
		diags = append(diags, DetectionDiagnostic{Code: "steam/proc-ownership-unknown", Subsystem: "steam", Source: "/proc", Outcome: OutcomeUnsupported, Detail: "process ownership unreadable; running state unknown"})
		return TriUnknown, diags
	}
	return TriNo, diags
}

func isPID(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func procUID(fs DetectionFS, pid string) (int, error) {
	raw, err := readBounded(fs, path.Join("/proc", pid, "status"), ReadLimitSmall)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(strings.TrimPrefix(line, "Uid:"))
			if len(fields) == 0 {
				return 0, errMalformed
			}
			return strconv.Atoi(fields[0])
		}
	}
	return 0, errMalformed
}
