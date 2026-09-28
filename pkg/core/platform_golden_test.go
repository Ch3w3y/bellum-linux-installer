package core

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// Golden matrix: pkg/core/testdata/platform/<case>/{rootfs/,env.json,
// gpu.json,links.json,expected.json,provenance.md}. Detection runs only
// through the injected fixture filesystem; scaffolding reads the fixture
// files from disk, the detector never does. expected.json files are reviewed
// diffs: they are generated once, read back by a human, and only then kept.

type goldenEnv struct {
	Vars     map[string]string `json:"vars"`
	Home     string            `json:"home"`
	EUID     int               `json:"euid"`
	HaveEUID bool              `json:"haveEUID"`
}

type goldenGPU struct {
	Renderer  string `json:"renderer"`
	Source    string `json:"source"`
	PCIVendor string `json:"pciVendor"`
	PCIDevice string `json:"pciDevice"`
	Fail      bool   `json:"fail"`
}

type goldenLinks struct {
	Links  map[string]string `json:"links"`
	Denied []string          `json:"denied"`
}

func loadGolden(t *testing.T, dir string) (DetectionSources, string) {
	t.Helper()
	var env goldenEnv
	readJSON(t, filepath.Join(dir, "env.json"), &env)
	var gpu goldenGPU
	readJSON(t, filepath.Join(dir, "gpu.json"), &gpu)
	var links goldenLinks
	if data, err := os.ReadFile(filepath.Join(dir, "links.json")); err == nil {
		if err := json.Unmarshal(data, &links); err != nil {
			t.Fatalf("%s: links.json: %v", dir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "provenance.md")); err != nil {
		t.Fatalf("%s: provenance.md is required for every golden case", dir)
	}
	fsys := newFixtureFS()
	root := filepath.Join(dir, "rootfs")
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		logical := "/" + filepath.ToSlash(rel)
		if d.IsDir() {
			fsys.addDir(logical)
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		fsys.addFile(logical, raw)
		return nil
	})
	for link, target := range links.Links {
		fsys.addLink(link, target)
	}
	for _, denied := range links.Denied {
		fsys.addDenied(denied)
	}
	vars := env.Vars
	if vars == nil {
		vars = map[string]string{}
	}
	provenance := "fixture:" + filepath.Base(dir)
	return DetectionSources{
		Files: fsys, Env: fixtureEnv(vars), Home: env.Home,
		GPU:      fixtureGPU(gpu.Renderer, gpu.Source, gpu.PCIVendor, gpu.PCIDevice, provenance, gpu.Fail),
		EUID:     env.EUID,
		HaveEUID: env.HaveEUID,
	}, provenance
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestPlatformGoldenMatrix(t *testing.T) {
	root := "testdata/platform"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("golden root: %v", err)
	}
	var cases []string
	for _, e := range entries {
		if e.IsDir() {
			cases = append(cases, e.Name())
		}
	}
	sort.Strings(cases)
	if len(cases) == 0 {
		t.Fatal("no golden cases")
	}
	dump := os.Getenv("BELLUM_DUMP_GOLDEN") == "1"
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			sources, _ := loadGolden(t, dir)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			platform, err := DetectPlatformWith(ctx, sources)
			if err != nil {
				t.Fatalf("golden detection failed: %v", err)
			}
			profile := ProfileFor(platform)
			actual, _ := json.Marshal(map[string]any{"platform": platform, "profile": profile, "profileKey": profile.Key()})
			expectedPath := filepath.Join(dir, "expected.json")
			if dump {
				var pretty bytes.Buffer
				_ = json.Indent(&pretty, actual, "", "  ")
				if err := os.WriteFile(expectedPath, []byte(pretty.String()+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
				t.Logf("dumped %s (REVIEW REQUIRED before keeping)", expectedPath)
				return
			}
			data, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("missing expected.json (generate with BELLUM_DUMP_GOLDEN=1, review, then keep): %v", err)
			}
			var want, got any
			if err := json.Unmarshal(data, &want); err != nil {
				t.Fatalf("expected.json: %v", err)
			}
			if err := json.Unmarshal(actual, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("golden mismatch for %s:\n got: %s", name, actual)
			}
		})
	}
}
