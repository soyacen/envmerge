package renderer

import (
	"testing"

	"github.com/soyacen/envmerge/internal/merger"
)

func TestRenderPreservesCommentsAndGroups(t *testing.T) {
	m := &merger.MergedDocument{Variables: []*merger.Variable{
		{Key: "APP_NAME", Value: "muonflow", LeadingComments: []string{"# Application"}},
		{Key: "DB_HOST", Value: "localhost", LeadingComments: []string{"# MySQL"}, GroupStart: true},
		{Key: "PORT", Value: "8080", InlineComment: "the port"},
	}}
	r := New(Options{PreserveComments: true, PreserveBlankLines: true})
	got, err := r.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Application\nAPP_NAME=muonflow\n\n# MySQL\nDB_HOST=localhost\nPORT=8080 # the port\n"
	if string(got) != want {
		t.Errorf("render:\n got %q\nwant %q", got, want)
	}
}

func TestRenderStripsComments(t *testing.T) {
	m := &merger.MergedDocument{Variables: []*merger.Variable{
		{Key: "A", Value: "1", LeadingComments: []string{"# drop me"}, GroupStart: true},
		{Key: "B", Value: "2", InlineComment: "drop"},
	}}
	r := New(Options{PreserveComments: false, PreserveBlankLines: false})
	got, _ := r.Render(m)
	if string(got) != "A=1\nB=2\n" {
		t.Errorf("stripped render = %q", got)
	}
}
