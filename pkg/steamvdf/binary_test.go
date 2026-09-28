package steamvdf

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// steamWritten is shortcuts.vdf as the Steam client writes it for one
// non-Steam game (field order and types from a client-written file).
func steamWritten() []byte {
	var b bytes.Buffer
	w := func(parts ...any) {
		for _, p := range parts {
			switch v := p.(type) {
			case byte:
				b.WriteByte(v)
			case string:
				b.WriteString(v)
				b.WriteByte(0)
			case []byte:
				b.Write(v)
			}
		}
	}
	w(TypeMap, "shortcuts")
	w(TypeMap, "0")
	w(TypeInt32, "appid", []byte{0x12, 0x34, 0x56, 0xf8})
	w(TypeString, "AppName", "Some Game")
	w(TypeString, "Exe", `"/usr/bin/somegame"`)
	w(TypeString, "StartDir", `"/usr/bin/"`)
	w(TypeString, "icon", "")
	w(TypeInt32, "IsHidden", []byte{0, 0, 0, 0})
	w(TypeUint64, "Big", []byte{1, 2, 3, 4, 5, 6, 7, 8})
	w(TypeFloat, "Scale", []byte{0, 0, 0x80, 0x3f})
	w(TypeMap, "tags")
	w(TypeString, "0", "favorite")
	w(typeEnd)
	w(typeEnd)
	w(typeEnd)
	w(typeEnd)
	return b.Bytes()
}

func TestRoundTripIsByteExact(t *testing.T) {
	in := steamWritten()
	root, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(in, out) {
		t.Fatalf("round trip changed the file:\n in %x\nout %x", in, out)
	}
	entry := root.Child("shortcuts").Child("0")
	if entry.StringValue("appname") != "Some Game" || entry.Child("appid").Int != 0xf8563412 || entry.Child("Big").U64 != 0x0807060504030201 {
		t.Fatalf("unexpected parse: %+v", entry)
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	good := steamWritten()
	deep := bytes.Repeat([]byte{TypeMap, 'a', 0}, maxDepth+2)
	for name, data := range map[string][]byte{
		"truncated":        good[:len(good)-1],
		"trailing":         append(append([]byte{}, good...), 0x08),
		"unknown type":     {0x05, 'x', 0, 0x08},
		"unterminated":     {TypeString, 'x', 0, 'y'},
		"short int":        {TypeInt32, 'x', 0, 1, 2},
		"too deep":         deep,
		"too large":        make([]byte, MaxFileSize+1),
		"empty":            {},
		"missing root end": {TypeMap, 's', 0, 0x08},
	} {
		if _, err := Parse(data); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: got %v, want ErrMalformed", name, err)
		}
	}
}

func TestEncodeRejectsNUL(t *testing.T) {
	root := NewDocument()
	root.Child("shortcuts").Children = append(root.Child("shortcuts").Children, String("x", "a\x00b"))
	if _, err := Encode(root); !errors.Is(err, ErrMalformed) {
		t.Fatalf("got %v", err)
	}
}

func TestAddAndRemoveShortcut(t *testing.T) {
	root, err := Parse(steamWritten())
	if err != nil {
		t.Fatal(err)
	}
	s := Shortcut{AppName: "Bellum", Exe: "/home/deck/.local/bin/Bellum", StartDir: "/home/deck/.local/bin", Icon: "/home/deck/.local/share/icons/hicolor/256x256/apps/bellum.png"}
	if !AddShortcut(root, s) {
		t.Fatal("not added")
	}
	if AddShortcut(root, s) {
		t.Fatal("added twice")
	}
	entry := root.Child("shortcuts").Child("1")
	if entry == nil || entry.StringValue("Exe") != `"/home/deck/.local/bin/Bellum"` || entry.Child("appid").Int&0x80000000 == 0 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Child("appid").Int != ShortcutAppID(`"/home/deck/.local/bin/Bellum"`, "Bellum") {
		t.Fatal("appid does not match Steam's derivation")
	}
	// The document still round-trips through the strict parser.
	data, err := Encode(root)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, err := Parse(data)
	if err != nil || !HasShortcut(reparsed, s.Exe) {
		t.Fatalf("reparse: %v", err)
	}

	// Removal only takes entries matching both the exe and the name.
	other := Shortcut{AppName: "Not Bellum", Exe: s.Exe, StartDir: s.StartDir}
	reparsed.Child("shortcuts").Children = append(reparsed.Child("shortcuts").Children, Map("7", String("AppName", other.AppName), String("Exe", Quote(other.Exe))))
	if n := RemoveShortcuts(reparsed, s.Exe, "Bellum"); n != 1 {
		t.Fatalf("removed %d", n)
	}
	m := reparsed.Child("shortcuts")
	var names []string
	for _, e := range m.Children {
		names = append(names, e.Name+"="+e.StringValue("AppName"))
	}
	if got := strings.Join(names, ","); got != "0=Some Game,1=Not Bellum" {
		t.Fatalf("after removal: %s", got)
	}
}

func TestNewDocument(t *testing.T) {
	root := NewDocument()
	AddShortcut(root, Shortcut{AppName: "Bellum", Exe: "/b", StartDir: "/"})
	data, err := Encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("\x00shortcuts\x00\x000\x00")) || !bytes.HasSuffix(data, []byte{0x08, 0x08, 0x08}) {
		t.Fatalf("unexpected encoding %x", data)
	}
}

// Any input either fails with ErrMalformed or round-trips byte for byte.
func FuzzParse(f *testing.F) {
	f.Add(steamWritten())
	f.Add([]byte{0x08})
	f.Add([]byte{TypeMap, 0, TypeString, 0, 0, 0x08, 0x08})
	f.Fuzz(func(t *testing.T, data []byte) {
		root, err := Parse(data)
		if err != nil {
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("unexpected error type: %v", err)
			}
			return
		}
		out, err := Encode(root)
		if err != nil || !bytes.Equal(out, data) {
			t.Fatalf("round trip: %v\n in %x\nout %x", err, data, out)
		}
	})
}
