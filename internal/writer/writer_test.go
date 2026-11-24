package writer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

func TestWriteNewFileAndPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".api.env")
	w := New()
	if err := w.Write(context.Background(), path, []byte("A=1\n"), false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "A=1\n" {
		t.Errorf("content = %q", data)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o, want 0600", fi.Mode().Perm())
	}
}

func TestWriteRefusesExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".api.env")
	if err := os.WriteFile(path, []byte("OLD"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := New().Write(context.Background(), path, []byte("NEW"), false)
	if diagnostic.CodeOf(err) != diagnostic.ErrOutputExists {
		t.Fatalf("expected ERR_OUTPUT_EXISTS, got %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "OLD" {
		t.Errorf("existing file must be preserved, got %q", data)
	}
}

func TestWriteForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".api.env")
	_ = os.WriteFile(path, []byte("OLD"), 0o600)
	if err := New().Write(context.Background(), path, []byte("NEW"), true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "NEW" {
		t.Errorf("force should overwrite, got %q", data)
	}
}

func TestNoTempFilesLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.env")
	_ = New().Write(context.Background(), path, []byte("A=1\n"), false)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if matched, _ := filepath.Match(".envmerge-*.tmp", e.Name()); matched {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestWriteCreatesNestedDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy", "env", ".api.env")
	if err := New().Write(context.Background(), path, []byte("A=1\n"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("nested output not written: %v", err)
	}
}

func TestWriteHonorsContextCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.env")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := New().Write(ctx, path, []byte("A=1\n"), false); err == nil {
		t.Error("cancelled context should prevent writing")
	}
}
