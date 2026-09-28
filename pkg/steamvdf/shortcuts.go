package steamvdf

import (
	"hash/crc32"
	"strconv"
	"strings"
)

// Shortcut is one non-Steam game entry.
type Shortcut struct {
	AppName  string
	Exe      string // the executable path, unquoted
	StartDir string // unquoted
	Icon     string
}

// Quote wraps a path the way Steam stores Exe and StartDir.
func Quote(path string) string { return `"` + path + `"` }

// ShortcutAppID is the ID Steam derives for a non-Steam shortcut from its
// quoted executable and name (the same value Steam ROM Manager and BoilR use).
func ShortcutAppID(quotedExe, appName string) uint32 {
	return crc32.ChecksumIEEE([]byte(quotedExe+appName)) | 0x80000000
}

// NewDocument returns an empty shortcuts.vdf document.
func NewDocument() *Node {
	return &Node{Type: TypeMap, Children: []*Node{Map("shortcuts")}}
}

// shortcutsMap returns the document's "shortcuts" map, adding it if missing.
func shortcutsMap(root *Node) *Node {
	if m := root.Child("shortcuts"); m != nil && m.Type == TypeMap {
		return m
	}
	m := Map("shortcuts")
	root.Children = append(root.Children, m)
	return m
}

// HasShortcut reports whether an entry launches exe.
func HasShortcut(root *Node, exe string) bool {
	m := root.Child("shortcuts")
	if m == nil || m.Type != TypeMap {
		return false
	}
	for _, e := range m.Children {
		if e.Type == TypeMap && sameExe(e.StringValue("Exe"), exe) {
			return true
		}
	}
	return false
}

func sameExe(stored, exe string) bool {
	return strings.Trim(stored, `"`) == exe
}

// AddShortcut appends s unless an entry for the same executable exists. It
// reports whether the document changed.
func AddShortcut(root *Node, s Shortcut) bool {
	if HasShortcut(root, s.Exe) {
		return false
	}
	m := shortcutsMap(root)
	next := 0
	for _, e := range m.Children {
		if n, err := strconv.Atoi(e.Name); err == nil && n >= next {
			next = n + 1
		}
	}
	exe := Quote(s.Exe)
	entry := Map(strconv.Itoa(next),
		Int32("appid", ShortcutAppID(exe, s.AppName)),
		String("AppName", s.AppName),
		String("Exe", exe),
		String("StartDir", Quote(s.StartDir)),
		String("icon", s.Icon),
		String("ShortcutPath", ""),
		String("LaunchOptions", ""),
		Int32("IsHidden", 0),
		Int32("AllowDesktopConfig", 1),
		Int32("AllowOverlay", 1),
		Int32("OpenVR", 0),
		Int32("Devkit", 0),
		String("DevkitGameID", ""),
		Int32("DevkitOverrideAppID", 0),
		Int32("LastPlayTime", 0),
		String("FlatpakAppID", ""),
		Map("tags"),
	)
	m.Children = append(m.Children, entry)
	return true
}

// RemoveShortcuts deletes every entry that launches exe and is named
// appName, renumbering the rest in order as Steam does. It returns how many were
// removed.
func RemoveShortcuts(root *Node, exe, appName string) int {
	m := root.Child("shortcuts")
	if m == nil || m.Type != TypeMap {
		return 0
	}
	kept := m.Children[:0]
	removed := 0
	for _, e := range m.Children {
		if e.Type == TypeMap && sameExe(e.StringValue("Exe"), exe) && e.StringValue("AppName") == appName {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	m.Children = kept
	if removed > 0 {
		for i, e := range m.Children {
			e.Name = strconv.Itoa(i)
		}
	}
	return removed
}
