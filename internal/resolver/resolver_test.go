package resolver_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/soyacen/envmerge/internal/diagnostic"
	"github.com/soyacen/envmerge/internal/loader"
	"github.com/soyacen/envmerge/internal/resolver"
)

// newResolver builds a resolver over an in-memory tree. Keys are fs.ValidPath
// style (no leading slash); includes resolve relative to the declaring file.
func newResolver(files map[string]string, maxDepth int, root string) *resolver.Engine {
	m := fstest.MapFS{}
	for name, content := range files {
		m[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return resolver.New(&loader.FSLoader{FS: m, Root: "/"}, maxDepth, root)
}

func TestSharedDiamondResolves(t *testing.T) {
	e := newResolver(map[string]string{
		"api.env.example":   "include ./db.env.example\ninclude ./redis.env.example\n",
		"db.env.example":    "include ./base.env.example\nDB=1\n",
		"redis.env.example": "include ./base.env.example\nREDIS=1\n",
		"base.env.example":  "APP_NAME=muonflow\n",
	}, 64, "")
	rd, err := e.Resolve(context.Background(), "/api.env.example")
	if err != nil {
		t.Fatalf("diamond should resolve: %v", err)
	}
	if deps := resolver.Dependencies(rd); len(deps) != 3 {
		t.Errorf("expected 3 unique dependencies, got %d: %#v", len(deps), deps)
	}
}

func TestHiddenFileInclude(t *testing.T) {
	e := newResolver(map[string]string{
		"api.env.example":           "include ./.mysql-hidden.env.example\n",
		".mysql-hidden.env.example": "SECRET=hunter2\n",
	}, 64, "")
	if _, err := e.Resolve(context.Background(), "/api.env.example"); err != nil {
		t.Fatalf("hidden include should load: %v", err)
	}
}

func TestCircularDirect(t *testing.T) {
	e := newResolver(map[string]string{
		"a.env.example": "include ./b.env.example\nA=1\n",
		"b.env.example": "include ./a.env.example\nB=2\n",
	}, 64, "")
	_, err := e.Resolve(context.Background(), "/a.env.example")
	if diagnostic.CodeOf(err) != diagnostic.ErrCircularInclude {
		t.Fatalf("expected circular, got %v", err)
	}
}

func TestCircularIndirect(t *testing.T) {
	e := newResolver(map[string]string{
		"a.env.example": "include ./b.env.example\nA=1\n",
		"b.env.example": "include ./c.env.example\nB=2\n",
		"c.env.example": "include ./a.env.example\nC=3\n",
	}, 64, "")
	_, err := e.Resolve(context.Background(), "/a.env.example")
	if diagnostic.CodeOf(err) != diagnostic.ErrCircularInclude {
		t.Fatalf("expected circular, got %v", err)
	}
	if !strings.Contains(err.Error(), "a.env.example") {
		t.Errorf("circular path should mention the file: %v", err)
	}
}

func TestMaxDepth(t *testing.T) {
	e := newResolver(map[string]string{
		"a.env.example": "include ./b.env.example\n",
		"b.env.example": "include ./c.env.example\n",
		"c.env.example": "C=3\n",
	}, 1, "")
	_, err := e.Resolve(context.Background(), "/a.env.example")
	if diagnostic.CodeOf(err) != diagnostic.ErrMaxDepth {
		t.Fatalf("expected ERR_MAX_DEPTH, got %v", err)
	}
}

func TestMissingInclude(t *testing.T) {
	e := newResolver(map[string]string{"a.env.example": "include ./nope.env.example\n"}, 64, "")
	_, err := e.Resolve(context.Background(), "/a.env.example")
	if diagnostic.CodeOf(err) != diagnostic.ErrFileNotFound {
		t.Fatalf("expected ERR_FILE_NOT_FOUND, got %v", err)
	}
}

func TestOptionalMissing(t *testing.T) {
	e := newResolver(map[string]string{"a.env.example": "include? ./nope.env.example\nA=1\n"}, 64, "")
	if _, err := e.Resolve(context.Background(), "/a.env.example"); err != nil {
		t.Fatalf("optional include must not fail: %v", err)
	}
}

func TestTreeCounts(t *testing.T) {
	e := newResolver(map[string]string{
		"api.env.example":  "include ./base.env.example\ninclude ./db.env.example\n",
		"base.env.example": "A=1\n",
		"db.env.example":   "include ./base.env.example\nDB=1\n",
	}, 64, "")
	rd, err := e.Resolve(context.Background(), "/api.env.example")
	if err != nil {
		t.Fatal(err)
	}
	if tree := resolver.Tree(rd); len(tree.Children) != 2 {
		t.Errorf("root should have 2 children, got %d", len(tree.Children))
	}
}
