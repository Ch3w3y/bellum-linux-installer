package core

import (
	"strings"
)

// OS release parsing lives in core (moved from workflow) so there is exactly
// one parser. The file is read as data: assignments and documented
// quoting/escaping are parsed, never sourced, and variables are never
// expanded.

// osReleasePaths are tried in order. /usr/lib/os-release is used only when
// /etc/os-release does not exist; the files are never merged.
var osReleasePaths = []string{"/etc/os-release", "/usr/lib/os-release"}

// idAlias maps an explicit ID (or an ID_LIKE token) to its package family.
// Unknown IDs fall through to ordered ID_LIKE tokens; there is no substring
// matching against arbitrary distro names.
var idAlias = map[string]OSFamily{
	"steamos":             OSArch,
	"arch":                OSArch,
	"cachyos":             OSArch,
	"endeavouros":         OSArch,
	"manjaro":             OSArch,
	"bazzite":             OSFedora,
	"fedora":              OSFedora,
	"nobara":              OSFedora,
	"rhel":                OSFedora,
	"centos":              OSFedora,
	"debian":              OSDebian,
	"ubuntu":              OSDebian,
	"linuxmint":           OSDebian,
	"mint":                OSDebian,
	"pop":                 OSDebian,
	"pop!_os":             OSDebian,
	"opensuse":            OSOpenSUSE,
	"opensuse-leap":       OSOpenSUSE,
	"opensuse-tumbleweed": OSOpenSUSE,
	"opensuse-microos":    OSOpenSUSE,
	"opensuse-slowroll":   OSOpenSUSE,
	"suse":                OSOpenSUSE,
	"sled":                OSOpenSUSE,
	"sles":                OSOpenSUSE,
}

// atomicVariants are recognized Fedora Atomic variant markers. Unrecognized
// variants remain unknown until evidence establishes their model.
var atomicVariants = map[string]bool{
	"atomic":     true,
	"silverblue": true,
	"kinoite":    true,
	"sericea":    true,
	"onyx":       true,
	"coreos":     true,
	"iot":        true,
}

// ParseOSRelease parses os-release content as data. It returns the known
// fields (lower-cased identity tokens, original pretty name) and a diagnostic
// when a repeated known key keeps only the final valid occurrence.
func ParseOSRelease(data []byte) (id, idLike, variantID, versionID, pretty string, idLikeTokens []string, diags []DetectionDiagnostic) {
	values := map[string]string{}
	seen := map[string]int{}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !validOSReleaseKey(key) {
			continue
		}
		parsed, ok := parseOSReleaseValue(strings.TrimSpace(value))
		if !ok {
			diags = append(diags, DetectionDiagnostic{
				Code:      "os-release/malformed-value",
				Subsystem: "os",
				Source:    "/etc/os-release",
				Outcome:   OutcomeMalformed,
				Detail:    SanitizeField("ignoring malformed assignment for "+key, MaxDetailLen),
			})
			continue
		}
		seen[key]++
		if seen[key] > 1 {
			switch key {
			case "ID", "ID_LIKE", "VARIANT_ID", "VERSION_ID", "PRETTY_NAME":
				diags = append(diags, DetectionDiagnostic{
					Code:      "os-release/duplicate-key",
					Subsystem: "os",
					Source:    "/etc/os-release",
					Outcome:   OutcomeConflict,
					Detail:    SanitizeField("repeated key "+key+"; using the final valid occurrence", MaxDetailLen),
				})
			}
		}
		values[key] = parsed
	}
	id = strings.ToLower(strings.TrimSpace(values["ID"]))
	idLike = strings.ToLower(strings.TrimSpace(values["ID_LIKE"]))
	variantID = strings.ToLower(strings.TrimSpace(values["VARIANT_ID"]))
	versionID = strings.TrimSpace(values["VERSION_ID"])
	pretty = strings.TrimSpace(values["PRETTY_NAME"])
	if idLike != "" {
		idLikeTokens = strings.Fields(idLike)
	}
	return id, idLike, variantID, versionID, pretty, idLikeTokens, diags
}

func validOSReleaseKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// parseOSReleaseValue handles bare, single-quoted and double-quoted values
// with shell-like backslash escapes inside double quotes. It never expands
// variables or runs command substitutions.
func parseOSReleaseValue(v string) (string, bool) {
	if v == "" {
		return "", true
	}
	switch v[0] {
	case '\'':
		if len(v) < 2 || v[len(v)-1] != '\'' {
			return "", false
		}
		return v[1 : len(v)-1], true
	case '"':
		if len(v) < 2 || v[len(v)-1] != '"' {
			return "", false
		}
		inner := v[1 : len(v)-1]
		var b strings.Builder
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\\' && i+1 < len(inner) {
				switch inner[i+1] {
				case '$', '"', '\\', '`':
					b.WriteByte(inner[i+1])
					i++
					continue
				case 'n':
					b.WriteByte('\n')
					i++
					continue
				}
			}
			b.WriteByte(inner[i])
		}
		return b.String(), true
	default:
		if strings.ContainsAny(v, "\"'") {
			return "", false
		}
		return v, true
	}
}

// ClassifyOSFamily resolves explicit ID aliases before ordered ID_LIKE
// tokens. SteamOS keeps its own ID (never rewritten to arch or bazzite to
// fedora); only the family is classified.
func ClassifyOSFamily(id string, idLikeTokens []string) OSFamily {
	if fam, ok := idAlias[id]; ok {
		return fam
	}
	for _, tok := range idLikeTokens {
		if fam, ok := idAlias[strings.ToLower(tok)]; ok {
			return fam
		}
	}
	return OSUnknown
}

// ClassifyImmutable determines the immutable state. SteamOS and Bazzite are
// immutable by known distro identity; recognized Fedora Atomic
// variants/ostree evidence set yes; unrecognized variants remain unknown
// until evidence establishes their model.
func ClassifyImmutable(id, variantID string, family OSFamily, ostreeBooted bool) (TriState, string) {
	switch id {
	case "steamos":
		return TriYes, "known distro identity: steamos"
	case "bazzite":
		return TriYes, "known distro identity: bazzite"
	}
	if ostreeBooted {
		return TriYes, "ostree boot marker present"
	}
	lower := strings.ToLower(variantID)
	if strings.Contains(lower, "immutable") || strings.Contains(lower, "atomic") {
		return TriYes, "variant marker: " + SanitizeField(variantID, 64)
	}
	if atomicVariants[lower] {
		return TriYes, "atomic variant: " + SanitizeField(variantID, 64)
	}
	if strings.TrimSpace(variantID) != "" {
		return TriUnknown, "unrecognized variant without model evidence"
	}
	if id == "" || family == OSUnknown {
		return TriUnknown, "no distro identity evidence"
	}
	return TriNo, "mutable distro model"
}
