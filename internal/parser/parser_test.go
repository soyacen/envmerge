package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

func nodes(t *testing.T, content string) *Document {
	t.Helper()
	doc, err := Parse("x.env.example", []byte(content))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	return doc
}

func kinds(doc *Document) []NodeKind {
	var out []NodeKind
	for _, n := range doc.Nodes {
		out = append(out, n.Kind)
	}
	return out
}

func TestParseBasicVariable(t *testing.T) {
	doc := nodes(t, "APP_NAME=muonflow\nPORT=8080\n")
	vars := 0
	for _, n := range doc.Nodes {
		if n.Kind == NodeVariable {
			vars++
		}
	}
	if vars != 2 {
		t.Fatalf("expected 2 variables, got %d (%v)", vars, kinds(doc))
	}
}

func TestParseKeepsQuotedAndSpecialValues(t *testing.T) {
	doc := nodes(t, `Q="hello world"
S='single'
EMPTY=
CONNECTION=mysql://root:pass#1@host/db?opt=1
URL=http://${HOST}:${PORT}
`)
	want := map[string]string{
		"Q":          `"hello world"`,
		"S":          `'single'`,
		"EMPTY":      ``,
		"CONNECTION": `mysql://root:pass#1@host/db?opt=1`,
		"URL":        `http://${HOST}:${PORT}`,
	}
	for _, n := range doc.Nodes {
		if n.Kind != NodeVariable {
			continue
		}
		got, ok := want[n.Key]
		if !ok {
			t.Fatalf("unexpected key %q", n.Key)
		}
		if n.Value != got {
			t.Errorf("%s: value = %q, want %q", n.Key, n.Value, got)
		}
		delete(want, n.Key)
	}
	for k := range want {
		t.Errorf("missing key %q", k)
	}
}

func TestParseInlineCommentSeparatesHash(t *testing.T) {
	doc := nodes(t, "PORT=8080 # the port\nOTHER=val #note\n")
	for _, n := range doc.Nodes {
		switch n.Key {
		case "PORT":
			if n.Value != "8080" || n.InlineComment != "the port" {
				t.Errorf("PORT value=%q inline=%q", n.Value, n.InlineComment)
			}
		case "OTHER":
			// "#note" has no preceding space, so it belongs to the value.
			if n.Value != "val" || n.InlineComment != "note" {
				t.Errorf("OTHER value=%q inline=%q", n.Value, n.InlineComment)
			}
		}
	}
}

func TestParseMultilineQuotedValue(t *testing.T) {
	doc := nodes(t, "PRIVATE_KEY=\"-----BEGIN\nLINE2\n-----END\"\nAFTER=1\n")
	var key *Node
	for i := range doc.Nodes {
		if doc.Nodes[i].Key == "PRIVATE_KEY" {
			key = &doc.Nodes[i]
		}
	}
	if key == nil {
		t.Fatal("PRIVATE_KEY not found")
	}
	if !strings.Contains(key.Value, "LINE2") {
		t.Errorf("multiline value not captured: %q", key.Value)
	}
	// AFTER=1 must still parse as the next statement (consumed lines correct).
	found := false
	for _, n := range doc.Nodes {
		if n.Kind == NodeVariable && n.Key == "AFTER" {
			found = true
		}
	}
	if !found {
		t.Error("AFTER not parsed after multiline value")
	}
}

func TestParseIncludeVariants(t *testing.T) {
	doc := nodes(t, "include ./base.env.example\ninclude \"./my config.env.example\"\ninclude ../common/x.env\n")
	includes := []string{}
	for _, n := range doc.Nodes {
		if n.Kind == NodeInclude {
			includes = append(includes, n.IncludePath)
		}
	}
	if strings.Join(includes, "|") != "./base.env.example|./my config.env.example|../common/x.env" {
		t.Errorf("include paths = %#v", includes)
	}
}

func TestParseOptionalInclude(t *testing.T) {
	doc := nodes(t, "include? ./local.env.example\n")
	if doc.Nodes[0].Kind != NodeInclude || !doc.Nodes[0].Optional {
		t.Errorf("expected optional include, got %+v", doc.Nodes[0])
	}
}

func TestCommentedIncludeIgnored(t *testing.T) {
	doc := nodes(t, "# include ./disabled.env.example\n")
	if len(doc.Nodes) != 1 || doc.Nodes[0].Kind != NodeComment {
		t.Fatalf("commented include should stay a comment, got %v", kinds(doc))
	}
}

func TestUnsetAndRequired(t *testing.T) {
	doc := nodes(t, "unset REDIS_PASSWORD\nrequired DB_PASSWORD\n")
	if doc.Nodes[0].Kind != NodeUnset || doc.Nodes[0].Key != "REDIS_PASSWORD" {
		t.Errorf("unset parsed wrong: %+v", doc.Nodes[0])
	}
	if doc.Nodes[1].Kind != NodeRequired || doc.Nodes[1].Key != "DB_PASSWORD" {
		t.Errorf("required parsed wrong: %+v", doc.Nodes[1])
	}
}

func TestCommentsAndBlankLinesAreNodesOfTheirOwn(t *testing.T) {
	doc := nodes(t, "# Header\nA=1\nB=2\n\n# Group\nC=3\n# dangling\n")
	want := []NodeKind{
		NodeComment, NodeVariable, NodeVariable,
		NodeBlank, NodeComment, NodeVariable, NodeComment,
	}
	if got := fmt.Sprint(kinds(doc)); got != fmt.Sprint(want) {
		t.Fatalf("node stream = %s, want %s", got, want)
	}
	if doc.Nodes[0].Text != "# Header" {
		t.Errorf("comment text = %q, want %q", doc.Nodes[0].Text, "# Header")
	}
	if doc.Nodes[len(doc.Nodes)-1].Text != "# dangling" {
		t.Errorf("dangling comment lost: %+v", doc.Nodes[len(doc.Nodes)-1])
	}
	for _, n := range doc.Nodes {
		if n.Kind == NodeVariable && n.Key == "A" && n.Position.Line != 2 {
			t.Errorf("A position = %+v, want line 2", n.Position)
		}
	}
}

func TestInvalidKeyAndNoEqualsAreErrors(t *testing.T) {
	for _, bad := range []string{"1BAD=x", "no equals sign here"} {
		_, err := Parse("x", []byte(bad))
		if err == nil {
			t.Errorf("expected error for %q", bad)
			continue
		}
		code := diagnostic.CodeOf(err)
		if code != diagnostic.ErrInvalidVariable && code != diagnostic.ErrInvalidSyntax {
			t.Errorf("%q: unexpected code %q", bad, code)
		}
	}
}

func TestShellDirectivesRejected(t *testing.T) {
	_, err := Parse("x", []byte("source ./a.env\n"))
	if diagnostic.CodeOf(err) != diagnostic.ErrInvalidSyntax {
		t.Fatalf("source should be rejected as invalid syntax, got %v", err)
	}
}

func TestShellSubstitutionNotExecuted(t *testing.T) {
	// $(...) stays a literal value, never executed.
	doc := nodes(t, "WHOAMI=$(whoami)\n")
	if doc.Nodes[0].Value != "$(whoami)" {
		t.Errorf("subshell must remain literal, got %q", doc.Nodes[0].Value)
	}
}

func TestBOMAndCRLFHandled(t *testing.T) {
	doc := nodes(t, "\ufeffA=1\r\nB=2\r\n")
	count := 0
	for _, n := range doc.Nodes {
		if n.Kind == NodeVariable {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("BOM/CRLF handling failed, vars=%d", count)
	}
}

func TestUnterminatedQuoteError(t *testing.T) {
	_, err := Parse("x", []byte("KEY=\"unclosed\nNEXT=1\n"))
	if diagnostic.CodeOf(err) != diagnostic.ErrInvalidSyntax {
		t.Fatalf("unterminated quote should error, got %v", err)
	}
}

func TestIncludePathErrors(t *testing.T) {
	for _, bad := range []string{"include\n", "include \"unterminated\n", "include ''\n"} {
		if _, err := Parse("x", []byte(bad)); err == nil {
			t.Errorf("expected include error for %q", bad)
		} else if diagnostic.CodeOf(err) != diagnostic.ErrInvalidInclude {
			t.Errorf("%q: expected ERR_INVALID_INCLUDE, got %v", bad, err)
		}
	}
}

func TestUnsetRequiredMissingName(t *testing.T) {
	for _, bad := range []string{"unset\n", "required\n"} {
		if _, err := Parse("x", []byte(bad)); diagnostic.CodeOf(err) != diagnostic.ErrInvalidSyntax {
			t.Errorf("%q should error, got %v", bad, err)
		}
	}
}

func TestSingleQuotedInclude(t *testing.T) {
	doc := nodes(t, "include './my dir/base.env'\n")
	if doc.Nodes[0].IncludePath != "./my dir/base.env" {
		t.Errorf("single-quoted include path = %q", doc.Nodes[0].IncludePath)
	}
}
