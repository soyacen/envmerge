package merger

import (
	"context"
	"strings"
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

// --- comment fidelity ----------------------------------------------------

// commentStream flattens the merged stream to the lines a renderer would print
// for comment and variable blocks, so tests can assert on template fidelity.
func commentStream(m *MergedDocument) []string {
	var out []string
	for _, b := range m.Blocks {
		switch b.Kind {
		case BlockComment:
			out = append(out, b.Text)
		case BlockVariable:
			out = append(out, b.Var.Key+"="+b.Var.Value)
		}
	}
	return out
}

func wantStream(t *testing.T, m *MergedDocument, want []string) {
	t.Helper()
	got := commentStream(m)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("stream:\n got %q\nwant %q", got, want)
	}
}

func TestEveryTemplateCommentSurvives(t *testing.T) {
	files := map[string]string{
		"api.env.example": "# service header\n" +
			"# second banner line\n" +
			"include ./catalogue.env.example\n" +
			"\n" +
			"# own section\n" +
			"OWN=1\n" +
			"# dangling note at end of file\n",
		"catalogue.env.example": "# documented defaults, no assignments here\n" +
			"# OWN=default\n" +
			"# OTHER=default\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	wantStream(t, m, []string{
		"# service header",
		"# second banner line",
		"# documented defaults, no assignments here",
		"# OWN=default",
		"# OTHER=default",
		"# own section",
		"OWN=1",
		"# dangling note at end of file",
	})
}

func TestCommentBeforeIncludeStaysAboveItsContent(t *testing.T) {
	files := map[string]string{
		"api.env.example": "# pulled in for the database\ninclude ./db.env.example\n",
		"db.env.example":  "DB_HOST=localhost\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	wantStream(t, m, []string{"# pulled in for the database", "DB_HOST=localhost"})
}

func TestLosingDeclarationCommentsAreRelocated(t *testing.T) {
	files := map[string]string{
		"api.env.example":  "include ./base.env.example\n# raised for local debugging\nLOG_LEVEL=debug\n",
		"base.env.example": "# Logging\nLOG_LEVEL=info\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	// The value is the override, the position stays where the key was first
	// seen, and the losing declaration's comment is relocated onto it.
	wantStream(t, m, []string{"# Logging", "LOG_LEVEL=debug"})

	var relocated []string
	for _, b := range m.Blocks {
		for _, run := range b.Moved {
			if run.From.Line != 2 {
				t.Errorf("moved run position = %+v, want the comment's own line 2", run.From)
			}
			relocated = append(relocated, run.Lines...)
		}
	}
	if strings.Join(relocated, "|") != "# raised for local debugging" {
		t.Errorf("losing comment not relocated: %#v", relocated)
	}
}

func TestRepeatedOverrideDoesNotStackRelocatedComments(t *testing.T) {
	files := map[string]string{
		"api.env.example":  "include ./base.env.example\n# raised\nLOG_LEVEL=debug\n# raised\nLOG_LEVEL=trace\n",
		"base.env.example": "# Logging\nLOG_LEVEL=info\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, b := range m.Blocks {
		for _, run := range b.Moved {
			for _, line := range run.Lines {
				if line == "# raised" {
					count++
				}
			}
		}
	}
	if count != 1 {
		t.Errorf("relocated comment lines = %d, want the run kept once", count)
	}
}

func TestUnsetKeepsCommentsAndDropsAssignment(t *testing.T) {
	files := map[string]string{
		"api.env.example":   "include ./redis.env.example\nunset REDIS_PASSWORD\n",
		"redis.env.example": "# Redis\nREDIS_HOST=localhost\n# supplied by the platform, never committed\nREDIS_PASSWORD=\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	wantStream(t, m, []string{"# Redis", "REDIS_HOST=localhost", "# supplied by the platform, never committed"})
	if strings.Join(m.Unset, ",") != "REDIS_PASSWORD" {
		t.Errorf("unset index = %v, want [REDIS_PASSWORD]", m.Unset)
	}
}

func TestDiamondContributesCommentsOnce(t *testing.T) {
	files := map[string]string{
		"api.env.example":  "include ./a.env.example\ninclude ./b.env.example\n",
		"a.env.example":    "include ./leaf.env.example\nA=1\n",
		"b.env.example":    "include ./leaf.env.example\nB=1\n",
		"leaf.env.example": "# shared leaf\nLEAF=1\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	wantStream(t, m, []string{"# shared leaf", "LEAF=1", "A=1", "B=1"})
}

func TestVariablesAndBlocksStayInSync(t *testing.T) {
	files := map[string]string{
		"api.env.example":  "# header\ninclude ./base.env.example\nPORT=8080\n",
		"base.env.example": "# app\nNAME=demo\n",
	}
	m, err := build(t, files, "/api.env.example", ConflictLocalWins)
	if err != nil {
		t.Fatal(err)
	}
	var inBlocks []*Variable
	for _, b := range m.Blocks {
		if b.Kind == BlockVariable {
			inBlocks = append(inBlocks, b.Var)
		}
	}
	if len(inBlocks) != len(m.Variables) {
		t.Fatalf("Blocks has %d variables, Variables has %d", len(inBlocks), len(m.Variables))
	}
	for i := range inBlocks {
		if inBlocks[i] != m.Variables[i] {
			t.Errorf("variable %d is a different object in Blocks and Variables", i)
		}
	}
}
