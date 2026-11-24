package loader

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

func TestFSLoaderReadsRelativeAndAbsolute(t *testing.T) {
	m := fstest.MapFS{
		"base.env.example": &fstest.MapFile{Data: []byte("A=1\n")},
	}
	l := &FSLoader{FS: m, Root: "/proj"}
	data, err := l.Load(context.Background(), "/proj/base.env.example")
	if err != nil || string(data) != "A=1\n" {
		t.Fatalf("load abs: %q err=%v", data, err)
	}
	data, err = l.Load(context.Background(), "base.env.example")
	if err != nil || string(data) != "A=1\n" {
		t.Fatalf("load rel: %q err=%v", data, err)
	}
	_, err = l.Load(context.Background(), "/proj/missing.env.example")
	if diagnostic.CodeOf(err) != diagnostic.ErrFileNotFound {
		t.Fatalf("expected ERR_FILE_NOT_FOUND, got %v", err)
	}
}

func TestJoinRelativeIsDeclarationScoped(t *testing.T) {
	got := JoinRelative("/a/b/x.env.example", "./c/y.env.example")
	if got != "/a/b/c/y.env.example" {
		t.Errorf("relative base wrong: %q", got)
	}
	if abs := JoinRelative("/a/b/x.env.example", "/opt/z.env"); abs != "/opt/z.env" {
		t.Errorf("absolute include should be unchanged: %q", abs)
	}
}

func TestCheckRoot(t *testing.T) {
	if err := CheckRoot("", "/anything"); err != nil {
		t.Errorf("empty root disables check, got %v", err)
	}
	base := t.TempDir()
	if err := CheckRoot(base, base+"/inside.env"); err != nil {
		t.Errorf("inside root should pass: %v", err)
	}
	if err := CheckRoot(base, "/etc/passwd"); diagnostic.CodeOf(err) != diagnostic.ErrPathOutsideRoot {
		t.Errorf("outside root should be rejected, got %v", err)
	}
}

func TestOSLoader(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/a.env.example"
	if err := os.WriteFile(path, []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := OSLoader{}
	data, err := l.Load(context.Background(), path)
	if err != nil || string(data) != "A=1\n" {
		t.Fatalf("OSLoader: %q err=%v", data, err)
	}
	_, err = l.Load(context.Background(), dir+"/missing.env")
	if diagnostic.CodeOf(err) != diagnostic.ErrFileNotFound {
		t.Errorf("expected not found, got %v", err)
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.Load(cancelCtx, path); err == nil {
		t.Error("cancelled context should error")
	}
}

func TestCanonicalIDStable(t *testing.T) {
	a := CanonicalID("x/y/../y/z.env.example")
	b := CanonicalID("x/y/z.env.example")
	if a != b {
		t.Errorf("canonicalization should normalize: %q vs %q", a, b)
	}
}
