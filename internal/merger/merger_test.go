package merger

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/soyacen/envmerge/internal/diagnostic"
	"github.com/soyacen/envmerge/internal/loader"
	"github.com/soyacen/envmerge/internal/resolver"
)

func build(t *testing.T, files map[string]string, entry string, policy ConflictPolicy) (*MergedDocument, error) {
	t.Helper()
	m := fstest.MapFS{}
	for name, content := range files {
		m[name] = &fstest.MapFile{Data: []byte(content)}
	}
	r := resolver.New(&loader.FSLoader{FS: m, Root: "/"}, 64, "")
	rd, err := r.Resolve(context.Background(), entry)
	if err != nil {
		return nil, err
	}
	return New().Merge(rd, policy)
}

func ordered(m *MergedDocument) []string {
	if m == nil {
		return nil
	}
	var out []string
	for _, v := range m.Variables {
		out = append(out, v.Key+"="+v.Value)
	}
	return out
}

func TestProximityVsOrderPolicies(t *testing.T) {
	files := map[string]string{
		"api.env.example": "include ./A.env.example\ninclude ./B.env.example\n",
		"A.env.example":   "V=aA\n",
		"B.env.example":   "include ./C.env.example\n",
		"C.env.example":   "V=cC\n",
	}
	// A is depth1; C is depth2 but declared later in the traversal.
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	if got := ordered(m); len(got) != 1 || got[0] != "V=aA" {
		t.Errorf("local-wins (proximity): got %v, want [V=aA]", got)
	}
	m, _ = build(t, files, "/api.env.example", ConflictLastWins)
	if got := ordered(m); len(got) != 1 || got[0] != "V=cC" {
		t.Errorf("last-wins: got %v, want [V=cC]", got)
	}
	m, _ = build(t, files, "/api.env.example", ConflictFirstWins)
	if got := ordered(m); len(got) != 1 || got[0] != "V=aA" {
		t.Errorf("first-wins: got %v, want [V=aA]", got)
	}
	if _, err := build(t, files, "/api.env.example", ConflictError); diagnostic.CodeOf(err) != diagnostic.ErrDuplicateVariable {
		t.Errorf("error policy should reject duplicates, got %v", err)
	}
}

func TestOutputOrderIsFirstAppearance(t *testing.T) {
	files := map[string]string{
		"api.env.example":  "include ./base.env.example\nHTTP=8080\n",
		"base.env.example": "LOG=info\nHTTP=80\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	if got := ordered(m); len(got) != 2 || got[0] != "LOG=info" || got[1] != "HTTP=8080" {
		t.Errorf("order = %v, want [LOG=info HTTP=8080]", got)
	}
}

func TestOverrideRecorded(t *testing.T) {
	files := map[string]string{
		"api.env.example":  "include ./base.env.example\nHTTP=8080\n",
		"base.env.example": "HTTP=80\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Overrides) != 1 || m.Overrides[0].Key != "HTTP" {
		t.Fatalf("expected 1 override, got %#v", m.Overrides)
	}
}

func TestUnsetRemovesKey(t *testing.T) {
	files := map[string]string{
		"api.env.example":  "include ./base.env.example\nunset REDIS_PASSWORD\n",
		"base.env.example": "REDIS_HOST=h\nREDIS_PASSWORD=p\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Variables) != 1 || m.Variables[0].Key != "REDIS_HOST" {
		t.Errorf("unset failed: %#v", ordered(m))
	}
}

func TestRequiredValidation(t *testing.T) {
	missing := map[string]string{
		"api.env.example": "required DB_PASSWORD\nA=1\n",
	}
	if _, err := build(t, missing, "/api.env.example", ConflictLocalWins); diagnostic.CodeOf(err) != diagnostic.ErrRequiredMissing {
		t.Errorf("required-missing not detected: %v", err)
	}
	emptyVal := map[string]string{
		"api.env.example": "include ./db.env.example\nrequired DB_PASSWORD\n",
		"db.env.example":  "DB_PASSWORD=\n",
	}
	if _, err := build(t, emptyVal, "/api.env.example", ConflictLocalWins); diagnostic.CodeOf(err) != diagnostic.ErrRequiredMissing {
		t.Errorf("required-empty not detected: %v", err)
	}
}
