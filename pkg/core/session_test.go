package core

import (
	"testing"
)

func TestClassifySessionPrecedence(t *testing.T) {
	// Gamescope wins over everything.
	got := ClassifySession(map[string]string{
		"XDG_CURRENT_DESKTOP": "gamescope", "WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0",
	})
	if got.Kind != SessionGamescope || got.GameMode != TriYes {
		t.Fatalf("gamescope desktop: %+v", got)
	}
	// Wayland before X11.
	got = ClassifySession(map[string]string{
		"XDG_CURRENT_DESKTOP": "KDE", "WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0",
	})
	if got.Kind != SessionWayland || got.Desktop != "KDE" || got.GameMode != TriNo {
		t.Fatalf("wayland: %+v", got)
	}
	got = ClassifySession(map[string]string{"XDG_CURRENT_DESKTOP": "GNOME", "DISPLAY": ":0"})
	if got.Kind != SessionX11 || got.Desktop != "GNOME" {
		t.Fatalf("x11: %+v", got)
	}
	got = ClassifySession(map[string]string{})
	if got.Kind != SessionHeadless {
		t.Fatalf("headless: %+v", got)
	}
}

func TestClassifySessionGamescopeDisplayAloneIsUnknownMode(t *testing.T) {
	got := ClassifySession(map[string]string{
		"XDG_CURRENT_DESKTOP": "KDE", "GAMESCOPE_WAYLAND_DISPLAY": "gamescope-0",
	})
	if got.Kind != SessionGamescope || got.GameMode != TriUnknown {
		t.Fatalf("nested compositor: %+v", got)
	}
}

func TestClassifySessionColonTokens(t *testing.T) {
	got := ClassifySession(map[string]string{
		"XDG_CURRENT_DESKTOP": "ubuntu:GNOME", "DISPLAY": ":0",
	})
	if got.Kind != SessionX11 || got.Desktop != "ubuntu" {
		t.Fatalf("colon desktop: %+v", got)
	}
	got = ClassifySession(map[string]string{
		"XDG_CURRENT_DESKTOP": "KDE:gamescope", "WAYLAND_DISPLAY": "wayland-0",
	})
	if got.Kind != SessionGamescope || got.GameMode != TriYes {
		t.Fatalf("colon gamescope token: %+v", got)
	}
}

func TestClassifySessionWaylandSessionType(t *testing.T) {
	got := ClassifySession(map[string]string{"XDG_SESSION_TYPE": "Wayland"})
	if got.Kind != SessionWayland {
		t.Fatalf("session-type wayland: %+v", got)
	}
}
