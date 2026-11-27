package renderer

import (
	"testing"

	"github.com/soyacen/envmerge/internal/diagnostic"
	"github.com/soyacen/envmerge/internal/merger"
)

func TestRenderPreservesCommentAndBlankBlocks(t *testing.T) {
	m := &merger.MergedDocument{Blocks: []merger.Block{
		{Kind: merger.BlockComment, Text: "# Application"},
		{Kind: merger.BlockVariable, Var: &merger.Variable{Key: "APP_NAME", Value: "muonflow"}},
		{Kind: merger.BlockBlank},
		{Kind: merger.BlockComment, Text: "# MySQL", GroupStart: true},
		{Kind: merger.BlockVariable, Var: &merger.Variable{Key: "DB_HOST", Value: "localhost"}},
		{
			Kind:  merger.BlockVariable,
			Var:   &merger.Variable{Key: "PORT", Value: "8080", InlineComment: "the port"},
			Moved: []merger.MovedRun{{From: diagnostic.Position{File: "api.env.example", Line: 12}, Lines: []string{"# raised locally"}}},
		},
	}}
	got, err := New().Render(m)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Application\nAPP_NAME=muonflow\n\n# MySQL\nDB_HOST=localhost\n# raised locally\nPORT=8080 # the port\n"
	if string(got) != want {
		t.Errorf("render:\n got %q\nwant %q", got, want)
	}
}

func TestRenderCollapsesBlankRunsAndTrimsEdges(t *testing.T) {
	m := &merger.MergedDocument{Blocks: []merger.Block{
		{Kind: merger.BlockBlank},
		{Kind: merger.BlockComment, Text: "# leading blanks must not open the file"},
		{Kind: merger.BlockBlank},
		{Kind: merger.BlockBlank},
		{Kind: merger.BlockVariable, Var: &merger.Variable{Key: "A", Value: "1"}},
		{Kind: merger.BlockBlank},
		{Kind: merger.BlockComment, Text: "# trailing note", GroupStart: true},
	}}
	got, err := New().Render(m)
	if err != nil {
		t.Fatal(err)
	}
	want := "# leading blanks must not open the file\n\nA=1\n\n# trailing note\n"
	if string(got) != want {
		t.Errorf("render:\n got %q\nwant %q", got, want)
	}
}

func TestRenderEmptyStreamProducesNoBytes(t *testing.T) {
	for _, m := range []*merger.MergedDocument{
		{},
		{Blocks: []merger.Block{{Kind: merger.BlockBlank}, {Kind: merger.BlockBlank}}},
	} {
		got, err := New().Render(m)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("stream %v rendered %q, want no bytes", m.Blocks, got)
		}
	}
}

func TestRenderReproducesMultilineQuotedValueVerbatim(t *testing.T) {
	m := &merger.MergedDocument{Blocks: []merger.Block{
		{Kind: merger.BlockComment, Text: "# PEM stays as declared"},
		{Kind: merger.BlockVariable, Var: &merger.Variable{Key: "PRIVATE_KEY", Value: "\"-----BEGIN\nMII\n-----END\""}},
	}}
	got, err := New().Render(m)
	if err != nil {
		t.Fatal(err)
	}
	want := "# PEM stays as declared\nPRIVATE_KEY=\"-----BEGIN\nMII\n-----END\"\n"
	if string(got) != want {
		t.Errorf("render:\n got %q\nwant %q", got, want)
	}
}
