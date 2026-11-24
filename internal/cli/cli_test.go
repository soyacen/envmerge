package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run executes the CLI with captured output and returns (code, stdout, stderr).
func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var so, se bytes.Buffer
	oldOut, oldErr := stdout, stderr
	stdout, stderr = &so, &se
	defer func() { stdout, stderr = oldOut, oldErr }()

	code := Execute(context.Background(), append([]string{"envmerge"}, args...))
	return code, so.String(), se.String()
}

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"template/base.env.example": "APP_NAME=muonflow\nLOG_LEVEL=info\n",
		".api.env.example":          "include ./template/base.env.example\nHTTP_PORT=8080\nLOG_LEVEL=debug\n",
		"cyc/.a.env.example":        "include ./.b.env.example\nA=1\n",
		"cyc/.b.env.example":        "include ./.a.env.example\nB=2\n",
		"sec/.p.env.example":        "include ./db.env.example\n",
		"sec/db.env.example":        "DB_PASSWORD=super-secret\n",
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCLIGenerate(t *testing.T) {
	dir := fixture(t)
	chdir(t, dir)
	code, _, _ := run(t, "generate", ".api.env.example")
	if code != 0 {
		t.Fatalf("generate exit=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".api.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "LOG_LEVEL=debug") || !strings.Contains(string(data), "HTTP_PORT=8080") {
		t.Errorf("unexpected generated content:\n%s", data)
	}
}

func TestCLIDryRunWritesNothing(t *testing.T) {
	dir := fixture(t)
	chdir(t, dir)
	code, out, _ := run(t, "generate", ".api.env.example", "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run exit=%d", code)
	}
	if !strings.Contains(out, "LOG_LEVEL=debug") {
		t.Errorf("dry-run should print content, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".api.env")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create the output file")
	}
}

func TestCLIValidate(t *testing.T) {
	dir := fixture(t)
	chdir(t, dir)
	code, out, _ := run(t, "validate", ".api.env.example")
	if code != 0 || !strings.Contains(out, "✓ Template syntax valid") {
		t.Fatalf("validate code=%d out=%q", code, out)
	}
}

func TestCLICircularExits3(t *testing.T) {
	dir := fixture(t)
	chdir(t, dir)
	code, _, errOut := run(t, "generate", "cyc/.a.env.example", "-o", "cyc/a.env")
	if code != exitTemplate {
		t.Fatalf("expected exit 3, got %d (%s)", code, errOut)
	}
	if !strings.Contains(errOut, "ERR_CIRCULAR_INCLUDE") || !strings.Contains(errOut, "circular path") {
		t.Errorf("expected circular diagnostic, got %q", errOut)
	}
}

func TestCLICheckMismatchExits4(t *testing.T) {
	dir := fixture(t)
	chdir(t, dir)
	// generate then corrupt the output -> check must report drift.
	if code, _, _ := run(t, "generate", ".api.env.example"); code != 0 {
		t.Fatalf("setup generate failed: %d", code)
	}
	_ = os.WriteFile(filepath.Join(dir, ".api.env"), []byte("STALE=1\n"), 0o600)
	code, _, _ := run(t, "check", ".api.env.example")
	if code != exitCheckDiff {
		t.Fatalf("expected exit 4, got %d", code)
	}
}

func TestCLIExplainRedactsSensitive(t *testing.T) {
	dir := fixture(t)
	chdir(t, dir)
	code, out, _ := run(t, "explain", "sec/.p.env.example", "DB_PASSWORD")
	if code != 0 {
		t.Fatalf("explain exit=%d", code)
	}
	if strings.Contains(out, "super-secret") {
		t.Errorf("sensitive value leaked in explain output: %q", out)
	}
	if !strings.Contains(out, "<redacted>") {
		t.Errorf("expected redaction marker, got %q", out)
	}
}

func TestCLITreeCounts(t *testing.T) {
	dir := fixture(t)
	chdir(t, dir)
	code, out, _ := run(t, "tree", ".api.env.example")
	if code != 0 {
		t.Fatalf("tree exit=%d", code)
	}
	if !strings.Contains(out, "template/base.env.example") || !strings.Contains(out, "Variables:") {
		t.Errorf("unexpected tree output:\n%s", out)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}
