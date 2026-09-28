// Package steamvdf reads and writes Steam's binary VDF format, as used by
// userdata/<account>/config/shortcuts.vdf (non-Steam games).
//
// The parser is strict and bounded: it accepts only the node types Steam
// writes, refuses truncated or trailing data, caps nesting depth and node
// count, and Encode reproduces a parsed file byte for byte, so a file is
// never rewritten in a form Steam didn't produce.
package steamvdf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// Node types in the binary format.
const (
	TypeMap    byte = 0x00
	TypeString byte = 0x01
	TypeInt32  byte = 0x02
	TypeFloat  byte = 0x03
	TypeUint64 byte = 0x07
	typeEnd    byte = 0x08
)

// Parser bounds. shortcuts.vdf holds one small map per shortcut.
const (
	MaxFileSize = 4 << 20
	maxDepth    = 16
	maxNodes    = 200000
)

// ErrMalformed marks input that is not a well-formed binary VDF document.
var ErrMalformed = errors.New("malformed binary VDF")

// Node is one key in a binary VDF document. Only the field matching Type is
// meaningful.
type Node struct {
	Type     byte
	Name     string
	Str      string
	Int      uint32
	U64      uint64
	Children []*Node
}

// Child returns the first child whose name matches case-insensitively, as
// Steam treats keys.
func (n *Node) Child(name string) *Node {
	for _, c := range n.Children {
		if strings.EqualFold(c.Name, name) {
			return c
		}
	}
	return nil
}

// StringValue returns a string child's value, or "".
func (n *Node) StringValue(name string) string {
	if c := n.Child(name); c != nil && c.Type == TypeString {
		return c.Str
	}
	return ""
}

// Parse decodes a whole document. The returned root is an unnamed map.
func Parse(data []byte) (*Node, error) {
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("%w: %d bytes exceeds %d", ErrMalformed, len(data), MaxFileSize)
	}
	p := parser{data: data}
	root := &Node{Type: TypeMap}
	if err := p.children(root, 0); err != nil {
		return nil, err
	}
	if p.pos != len(p.data) {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrMalformed, len(p.data)-p.pos)
	}
	return root, nil
}

type parser struct {
	data  []byte
	pos   int
	nodes int
}

func (p *parser) byte() (byte, error) {
	if p.pos >= len(p.data) {
		return 0, fmt.Errorf("%w: unexpected end of data", ErrMalformed)
	}
	b := p.data[p.pos]
	p.pos++
	return b, nil
}

func (p *parser) cstring() (string, error) {
	end := bytes.IndexByte(p.data[p.pos:], 0)
	if end < 0 {
		return "", fmt.Errorf("%w: unterminated string", ErrMalformed)
	}
	s := string(p.data[p.pos : p.pos+end])
	p.pos += end + 1
	return s, nil
}

func (p *parser) fixed(n int) ([]byte, error) {
	if len(p.data)-p.pos < n {
		return nil, fmt.Errorf("%w: unexpected end of data", ErrMalformed)
	}
	b := p.data[p.pos : p.pos+n]
	p.pos += n
	return b, nil
}

// children reads nodes into parent until the map's end marker.
func (p *parser) children(parent *Node, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("%w: nesting deeper than %d", ErrMalformed, maxDepth)
	}
	for {
		t, err := p.byte()
		if err != nil {
			return err
		}
		if t == typeEnd {
			return nil
		}
		if p.nodes++; p.nodes > maxNodes {
			return fmt.Errorf("%w: more than %d nodes", ErrMalformed, maxNodes)
		}
		name, err := p.cstring()
		if err != nil {
			return err
		}
		n := &Node{Type: t, Name: name}
		switch t {
		case TypeMap:
			if err := p.children(n, depth+1); err != nil {
				return err
			}
		case TypeString:
			if n.Str, err = p.cstring(); err != nil {
				return err
			}
		case TypeInt32, TypeFloat:
			b, err := p.fixed(4)
			if err != nil {
				return err
			}
			n.Int = binary.LittleEndian.Uint32(b)
		case TypeUint64:
			b, err := p.fixed(8)
			if err != nil {
				return err
			}
			n.U64 = binary.LittleEndian.Uint64(b)
		default:
			return fmt.Errorf("%w: unsupported node type 0x%02x", ErrMalformed, t)
		}
		parent.Children = append(parent.Children, n)
	}
}

// Encode serializes a root map produced by Parse or built by the caller.
// Names and string values must not contain NUL bytes.
func Encode(root *Node) ([]byte, error) {
	if root == nil || root.Type != TypeMap {
		return nil, fmt.Errorf("%w: root must be a map", ErrMalformed)
	}
	var buf bytes.Buffer
	if err := encodeChildren(&buf, root, 0); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeChildren(buf *bytes.Buffer, parent *Node, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("%w: nesting deeper than %d", ErrMalformed, maxDepth)
	}
	for _, n := range parent.Children {
		if strings.IndexByte(n.Name, 0) >= 0 || strings.IndexByte(n.Str, 0) >= 0 {
			return fmt.Errorf("%w: NUL byte in %q", ErrMalformed, n.Name)
		}
		buf.WriteByte(n.Type)
		buf.WriteString(n.Name)
		buf.WriteByte(0)
		switch n.Type {
		case TypeMap:
			if err := encodeChildren(buf, n, depth+1); err != nil {
				return err
			}
		case TypeString:
			buf.WriteString(n.Str)
			buf.WriteByte(0)
		case TypeInt32, TypeFloat:
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], n.Int)
			buf.Write(b[:])
		case TypeUint64:
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], n.U64)
			buf.Write(b[:])
		default:
			return fmt.Errorf("%w: unsupported node type 0x%02x", ErrMalformed, n.Type)
		}
	}
	buf.WriteByte(typeEnd)
	return nil
}

// String builds a string node.
func String(name, value string) *Node { return &Node{Type: TypeString, Name: name, Str: value} }

// Int32 builds a 32-bit integer node.
func Int32(name string, value uint32) *Node { return &Node{Type: TypeInt32, Name: name, Int: value} }

// Map builds a map node.
func Map(name string, children ...*Node) *Node {
	return &Node{Type: TypeMap, Name: name, Children: children}
}
