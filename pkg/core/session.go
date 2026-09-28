package core

import (
	"strings"
)

// Session classification lives in core beside platform detection. It reuses
// the existing DisplaySession precedence: gamescope signals first, then
// Wayland, then X11, then no graphical session. Desktop tokens match
// case-insensitively, including colon-separated XDG_CURRENT_DESKTOP values.

// ClassifySessionFunc classifies from an environment lookup.
func ClassifySessionFunc(env func(string) string) SessionInfo {
	return ClassifySession(map[string]string{
		"XDG_CURRENT_DESKTOP":       env("XDG_CURRENT_DESKTOP"),
		"GAMESCOPE_WAYLAND_DISPLAY": env("GAMESCOPE_WAYLAND_DISPLAY"),
		"WAYLAND_DISPLAY":           env("WAYLAND_DISPLAY"),
		"XDG_SESSION_TYPE":          env("XDG_SESSION_TYPE"),
		"DISPLAY":                   env("DISPLAY"),
	})
}

// ClassifySession is the pure session classifier over the relevant
// environment values.
func ClassifySession(env map[string]string) SessionInfo {
	desktop := SanitizeField(firstDesktopToken(env["XDG_CURRENT_DESKTOP"]), 128)
	hasGamescopeDesktop := desktopTokenPresent(env["XDG_CURRENT_DESKTOP"], "gamescope")
	switch {
	case hasGamescopeDesktop || strings.TrimSpace(env["GAMESCOPE_WAYLAND_DISPLAY"]) != "":
		// An explicit gamescope desktop session maps to Game Mode.
		// GAMESCOPE_WAYLAND_DISPLAY alone reports gamescope with GameMode
		// unknown because a nested desktop compositor can set it.
		if hasGamescopeDesktop {
			return SessionInfo{Kind: SessionGamescope, Desktop: desktop, GameMode: TriYes, Evidence: "xdg-current-desktop:gamescope"}
		}
		return SessionInfo{Kind: SessionGamescope, Desktop: desktop, GameMode: TriUnknown, Evidence: "gamescope-wayland-display-only"}
	case strings.TrimSpace(env["WAYLAND_DISPLAY"]) != "" || strings.EqualFold(strings.TrimSpace(env["XDG_SESSION_TYPE"]), "wayland"):
		return SessionInfo{Kind: SessionWayland, Desktop: desktop, GameMode: TriNo, Evidence: "wayland-display"}
	case strings.TrimSpace(env["DISPLAY"]) != "":
		return SessionInfo{Kind: SessionX11, Desktop: desktop, GameMode: TriNo, Evidence: "x11-display"}
	default:
		return SessionInfo{Kind: SessionHeadless, Desktop: desktop, GameMode: TriNo, Evidence: "no-display"}
	}
}

func firstDesktopToken(raw string) string {
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ':' || r == ';' }) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func desktopTokenPresent(raw, want string) bool {
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ':' || r == ';' }) {
		if strings.EqualFold(strings.TrimSpace(part), want) {
			return true
		}
	}
	return false
}
