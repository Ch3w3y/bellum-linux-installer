package core

import (
	"context"
	"path"
	"sort"
	"strings"
)

// Informational controller and process observations. Controllers never change
// the GPU profile, and an observed virtual controller never proves this game
// uses Steam Input. Event devices are never opened and no mappings are
// configured.

const inputDevicesPath = "/proc/bus/input/devices"
const inputByIDPath = "/dev/input/by-id"

var padNameTokens = []string{
	"gamepad", "controller", "joystick", "joypad", "joy-con",
	"xbox", "x-box", "playstation", "dualshock", "dualsense", "8bitdo",
	"steam controller", "steam deck", "nintendo", "switch pro",
	"stadia", "luna",
}

var nonPadTokens = []string{"keyboard", "mouse", "touchpad", "trackpoint"}

// DetectControllers parses /proc/bus/input/devices by records and optionally
// corroborates through /dev/input/by-id links. Absent by-id links never rule
// out Bluetooth or built-in devices.
func DetectControllers(ctx context.Context, fs DetectionFS) (ControllerInfo, []DetectionDiagnostic) {
	info := ControllerInfo{Discovery: DiscoveryComplete}
	var diags []DetectionDiagnostic
	add := func(code, source string, outcome DiagnosticOutcome, detail string) {
		diags = append(diags, DetectionDiagnostic{Code: code, Subsystem: "controllers", Source: source, Outcome: outcome, Detail: SanitizeField(detail, MaxDetailLen)})
	}

	raw, err := readBounded(fs, inputDevicesPath, ReadLimitText)
	if err != nil {
		info.Discovery = DiscoveryUnknown
		add("controllers/input-unreadable", inputDevicesPath, outcomeOf(err), "input device inventory unavailable")
		return info, diags
	}

	devices := parseInputDevices(string(raw))
	byID := readByIDHandlers(ctx, fs)
	for i := range devices {
		devices[i].Handlers = dedupeSorted(devices[i].Handlers)
		if byID != nil && devices[i].IdentityConfidence == ControllerCandidate && corroboratedByID(devices[i], byID) {
			devices[i].IdentityConfidence = ControllerPad
		}
		if err := ctx.Err(); err != nil {
			info.Discovery = DiscoveryPartial
			add("controllers/cancelled", inputDevicesPath, OutcomeTimeout, "controller inventory cancelled")
			break
		}
	}
	info.Devices = devices
	return info, diags
}

type inputRecord struct {
	name     string
	bus      string
	vendor   string
	product  string
	handlers []string
}

func parseInputDevices(text string) []ControllerDevice {
	var out []ControllerDevice
	for _, record := range strings.Split(text, "\n\n") {
		rec := parseInputRecord(record)
		if rec.name == "" && len(rec.handlers) == 0 {
			continue
		}
		conf, ok := classifyInputRecord(rec)
		if !ok {
			continue
		}
		out = append(out, ControllerDevice{
			Name:               SanitizeField(rec.name, MaxRawFieldLen),
			Bus:                SanitizeField(rec.bus, 16),
			VendorID:           SanitizeField(strings.ToLower(rec.vendor), 16),
			ProductID:          SanitizeField(strings.ToLower(rec.product), 16),
			Handlers:           rec.handlers,
			IdentityConfidence: conf,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].VendorID != out[j].VendorID {
			return out[i].VendorID < out[j].VendorID
		}
		if out[i].ProductID != out[j].ProductID {
			return out[i].ProductID < out[j].ProductID
		}
		return strings.Join(out[i].Handlers, ",") < strings.Join(out[j].Handlers, ",")
	})
	return out
}

func parseInputRecord(record string) inputRecord {
	var rec inputRecord
	for _, line := range strings.Split(record, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "N: Name="):
			rec.name = unquoteInputValue(strings.TrimPrefix(line, "N: Name="))
		case strings.HasPrefix(line, "I:"):
			for _, field := range strings.Fields(strings.TrimPrefix(line, "I:")) {
				switch {
				case strings.HasPrefix(field, "Bus="):
					rec.bus = strings.TrimPrefix(field, "Bus=")
				case strings.HasPrefix(field, "Vendor="):
					rec.vendor = strings.TrimPrefix(field, "Vendor=")
				case strings.HasPrefix(field, "Product="):
					rec.product = strings.TrimPrefix(field, "Product=")
				}
			}
		case strings.HasPrefix(line, "H: Handlers="):
			for _, h := range strings.Fields(strings.TrimPrefix(line, "H: Handlers=")) {
				rec.handlers = append(rec.handlers, h)
			}
		}
	}
	return rec
}

func unquoteInputValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		inner := v[1 : len(v)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `\\`, `\`)
		return inner
	}
	return v
}

// classifyInputRecord keeps pads and uncertain matches while excluding
// evident keyboards and mice. Two identical physical pads are never merged:
// every record stands on its own. (ok=false means excluded.)
func classifyInputRecord(rec inputRecord) (IdentityConfidence, bool) {
	hasJS := false
	for _, h := range rec.handlers {
		if strings.HasPrefix(h, "js") {
			hasJS = true
			break
		}
	}
	lower := strings.ToLower(rec.name)
	padLike := false
	for _, tok := range padNameTokens {
		if strings.Contains(lower, tok) {
			padLike = true
			break
		}
	}
	nonPad := false
	for _, tok := range nonPadTokens {
		if strings.Contains(lower, tok) {
			nonPad = true
			break
		}
	}
	switch {
	case hasJS && padLike:
		return ControllerPad, true
	case hasJS:
		return ControllerCandidate, true
	case padLike && !nonPad:
		return ControllerCandidate, true
	default:
		return "", false
	}
}

// readByIDHandlers maps handler basenames (event4, js0) seen through
// /dev/input/by-id links. Failures only remove corroboration, never devices.
func readByIDHandlers(ctx context.Context, fs DetectionFS) map[string]bool {
	names, err := readDirNames(fs, inputByIDPath)
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			break
		}
		target, lerr := fs.Readlink(path.Join(inputByIDPath, name))
		if lerr != nil {
			continue
		}
		base := path.Base(target)
		if base != "" {
			out[base] = true
		}
		// A pad-like link name corroborates even when the target basename
		// does not line up with a handler.
		lower := strings.ToLower(name)
		for _, tok := range padNameTokens {
			if strings.Contains(lower, tok) {
				out["padlink:"+name] = true
			}
		}
	}
	return out
}

func corroboratedByID(dev ControllerDevice, byID map[string]bool) bool {
	for _, h := range dev.Handlers {
		if byID[h] {
			return true
		}
	}
	lower := strings.ToLower(dev.Name)
	for key := range byID {
		if strings.HasPrefix(key, "padlink:") && strings.Contains(lower, "steam") {
			return true
		}
	}
	return false
}

func dedupeSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
