package core

import (
	"context"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// Fixture filesystem for platform golden tests. It translates logical
// absolute Linux paths into fixture-relative entries using only lexical path
// math: it never calls filepath.EvalSymlinks on the host, never reads the
// test process's HOME or environment, and never executes a probe. Symlinks,
// missing files, permission errors, cycles and escaping links are modeled
// explicitly so tests prove fixtures cannot escape to host files.

type fixtureEntry struct {
	data   []byte
	isDir  bool
	denied bool
}

type fixtureFS struct {
	entries map[string]*fixtureEntry
	links   map[string]string
}

type fixtureDirEntry struct {
	name  string
	isDir bool
}

func (e fixtureDirEntry) Name() string      { return e.name }
func (e fixtureDirEntry) IsDir() bool       { return e.isDir }
func (e fixtureDirEntry) Type() fs.FileMode { return 0 }
func (e fixtureDirEntry) Info() (fs.FileInfo, error) {
	return fixtureFileInfo{name: e.name, dir: e.isDir}, nil
}

type fixtureFileInfo struct {
	name string
	size int64
	dir  bool
}

func (i fixtureFileInfo) Name() string       { return i.name }
func (i fixtureFileInfo) Size() int64        { return i.size }
func (i fixtureFileInfo) Mode() fs.FileMode  { return 0444 }
func (i fixtureFileInfo) ModTime() time.Time { return time.Time{} }
func (i fixtureFileInfo) IsDir() bool        { return i.dir }
func (i fixtureFileInfo) Sys() any           { return nil }

func cleanLogical(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}

func newFixtureFS() *fixtureFS {
	return &fixtureFS{entries: map[string]*fixtureEntry{}, links: map[string]string{}}
}

func (f *fixtureFS) addFile(p string, data []byte) {
	p = cleanLogical(p)
	f.entries[p] = &fixtureEntry{data: append([]byte(nil), data...)}
	dir := path.Dir(p)
	for dir != "/" && dir != "." {
		if _, ok := f.entries[dir]; !ok {
			f.entries[dir] = &fixtureEntry{isDir: true}
		}
		dir = path.Dir(dir)
	}
	if _, ok := f.entries["/"]; !ok {
		f.entries["/"] = &fixtureEntry{isDir: true}
	}
}

func (f *fixtureFS) addDir(p string) {
	p = cleanLogical(p)
	if _, ok := f.entries[p]; !ok {
		f.entries[p] = &fixtureEntry{isDir: true}
	}
	dir := path.Dir(p)
	for dir != "/" && dir != "." {
		if _, ok := f.entries[dir]; !ok {
			f.entries[dir] = &fixtureEntry{isDir: true}
		}
		dir = path.Dir(dir)
	}
	if _, ok := f.entries["/"]; !ok {
		f.entries["/"] = &fixtureEntry{isDir: true}
	}
}

func (f *fixtureFS) addLink(p, target string) {
	f.links[cleanLogical(p)] = target
}

func (f *fixtureFS) addDenied(p string) {
	p = cleanLogical(p)
	f.entries[p] = &fixtureEntry{denied: true}
}

// resolve follows links lexically inside the fixture namespace. A target
// that is absent from the fixture resolves to a missing path (never to host
// content); a target flagged outside the namespace is an escape error.
func (f *fixtureFS) resolve(p string) (string, error) {
	current := cleanLogical(p)
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		if seen[current] {
			return "", &fs.PathError{Op: "readlink", Path: p, Err: errLinkCycle}
		}
		seen[current] = true
		target, isLink := f.links[current]
		if !isLink || target == "" {
			return current, nil
		}
		if strings.HasPrefix(target, "HOST:") {
			return "", &fs.PathError{Op: "readlink", Path: p, Err: errLinkEscape}
		}
		if strings.HasPrefix(target, "/") {
			current = cleanLogical(target)
		} else {
			current = cleanLogical(path.Join(path.Dir(current), target))
		}
	}
	return "", &fs.PathError{Op: "readlink", Path: p, Err: errLinkCycle}
}

func (f *fixtureFS) ReadFile(p string) ([]byte, error) {
	resolved, err := f.resolve(p)
	if err != nil {
		return nil, err
	}
	e, ok := f.entries[resolved]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}
	if e.denied {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrPermission}
	}
	if e.isDir {
		return nil, &fs.PathError{Op: "read", Path: p, Err: errMalformed}
	}
	return append([]byte(nil), e.data...), nil
}

func (f *fixtureFS) Stat(p string) (fs.FileInfo, error) {
	resolved, err := f.resolve(p)
	if err != nil {
		return nil, err
	}
	if target, isLink := f.links[cleanLogical(p)]; isLink && target == "" {
		return fixtureFileInfo{name: path.Base(resolved)}, nil
	}
	e, ok := f.entries[resolved]
	if !ok {
		// A dangling symlink target is missing, never host content.
		if _, isLink := f.links[cleanLogical(p)]; isLink {
			return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
		}
		return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
	}
	if e.denied {
		return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrPermission}
	}
	if e.isDir {
		return fixtureFileInfo{name: path.Base(resolved), dir: true}, nil
	}
	return fixtureFileInfo{name: path.Base(resolved), size: int64(len(e.data))}, nil
}

func (f *fixtureFS) ReadDir(dir string) ([]fs.DirEntry, error) {
	resolved, err := f.resolve(dir)
	if err != nil {
		return nil, err
	}
	e, ok := f.entries[resolved]
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fs.ErrNotExist}
	}
	if e.denied {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fs.ErrPermission}
	}
	if !e.isDir {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: errMalformed}
	}
	children := map[string]bool{}
	prefix := strings.TrimSuffix(resolved, "/") + "/"
	for name := range f.entries {
		if strings.HasPrefix(name, prefix) {
			rest := strings.TrimPrefix(name, prefix)
			if rest != "" && !strings.Contains(rest, "/") {
				children[rest] = f.entries[name].isDir
			}
		}
	}
	for link := range f.links {
		if strings.HasPrefix(link, prefix) {
			rest := strings.TrimPrefix(link, prefix)
			if rest != "" && !strings.Contains(rest, "/") {
				if _, dup := children[rest]; !dup {
					children[rest] = false
				}
			}
		}
	}
	var out []fs.DirEntry
	for name, isDir := range children {
		out = append(out, fixtureDirEntry{name: name, isDir: isDir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

func (f *fixtureFS) Readlink(p string) (string, error) {
	p = cleanLogical(p)
	target, ok := f.links[p]
	if !ok {
		return "", &fs.PathError{Op: "readlink", Path: p, Err: fs.ErrNotExist}
	}
	if target == "" {
		return "", &fs.PathError{Op: "readlink", Path: p, Err: errMalformed}
	}
	if strings.HasPrefix(target, "HOST:") {
		return "", &fs.PathError{Op: "readlink", Path: p, Err: errLinkEscape}
	}
	return target, nil
}

// fixtureGPU builds the fixture GPU callback: captured renderer text through
// the real step-1 classifier with explicit provenance. Generation is derived,
// never hard-coded.
func fixtureGPU(renderer, source, pciVendor, pciDevice, provenance string, fail bool) GPUProbeFunc {
	return func(ctx context.Context) (GPUCapabilities, GPUProbeInfo, error) {
		if err := ctx.Err(); err != nil {
			return GPUCapabilities{}, GPUProbeInfo{}, err
		}
		if fail {
			return GPUCapabilities{}, GPUProbeInfo{Source: ProbeUnknown, Provenance: provenance}, context.DeadlineExceeded
		}
		caps := ClassifyGPUCapabilities(renderer)
		caps.Renderer = renderer
		src := ProbeUnknown
		switch source {
		case "renderer":
			src = ProbeRenderer
		case "lspci":
			src = ProbeLspci
		case "drm":
			src = ProbeDRM
		}
		assoc := "unknown"
		if src == ProbeRenderer || src == ProbeLspci {
			assoc = "known"
		}
		return caps, GPUProbeInfo{Source: src, Renderer: renderer, PCIVendor: pciVendor, PCIDevice: pciDevice, DeviceAssociation: assoc, Provenance: provenance}, nil
	}
}

func fixtureEnv(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}
