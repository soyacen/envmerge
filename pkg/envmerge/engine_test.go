package envmerge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func goldenDirs(t *testing.T) []string {
	root := filepath.Join("..", "..", "testdata", "golden")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	if len(dirs) == 0 {
		t.Fatal("no golden cases found")
	}
	return dirs
}

func TestGoldenFiles(t *testing.T) {
	for _, dir := range goldenDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			input := filepath.Join(dir, "input.env.example")
			expected, err := os.ReadFile(filepath.Join(dir, "expected.env"))
			if err != nil {
				t.Fatal(err)
			}
			eng := New(Options{})
			res, err := eng.Render(context.Background(), input)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !bytes.Equal(res.Content, expected) {
				t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", res.Content, expected)
			}
		})
	}
}

func TestDeriveOutputPath(t *testing.T) {
	e := New(Options{})
	cases := map[string]string{
		".api.env.example":    ".api.env",
		"api.env.example":     "api.env",
		".env.example":        ".env",
		"config/.env.example": "config/.env",
		"worker.env.example":  "worker.env",
	}
	for in, want := range cases {
		if got := e.DeriveOutputPath(in); got != want {
			t.Errorf("derive %q = %q, want %q", in, got, want)
		}
	}
}

func TestIdempotentGenerate(t *testing.T) {
	dir := t.TempDir()
	// Copy the basic golden set into a temp dir so include paths resolve.
	src := filepath.Join("..", "..", "testdata", "golden", "basic")
	copyTree(t, src, dir)

	eng := New(Options{Force: true})
	input := filepath.Join(dir, "input.env.example")
	first, err := eng.Generate(context.Background(), input, filepath.Join(dir, "out.env"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := eng.Generate(context.Background(), input, filepath.Join(dir, "out.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Content, second.Content) {
		t.Errorf("repeated generation produced different output")
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			_ = os.MkdirAll(d, 0o755)
			copyTree(t, s, d)
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(d, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
