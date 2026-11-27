package parser

import (
	"strings"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

// Parse decodes template content into a Document. The returned nodes preserve
// original ordering, quoting, comments and blank lines. Every comment line and
// every blank line becomes a node of its own, so nothing a human wrote in a
// template is lost before the merge stage. Parse never expands variables and
// never executes shell syntax.
func Parse(path string, content []byte) (*Document, error) {
	p := &parser{path: path}
	return p.parse(content)
}

type parser struct {
	path string
}

// physicalLine pairs a line number with its text (CR/LF stripped).
type physicalLine struct {
	num  int
	text string
}

func (p *parser) parse(content []byte) (*Document, error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimPrefix(text, "\ufeff") // strip UTF-8 BOM

	lines := splitLines(text)
	doc := &Document{Path: p.path}

	for i := 0; i < len(lines); {
		ln := lines[i]
		trimmed := strings.TrimSpace(ln.text)
		pos := diagnostic.Position{File: p.path, Line: ln.num, Column: indentWidth(ln.text) + 1}

		if trimmed == "" {
			doc.Nodes = append(doc.Nodes, Node{Kind: NodeBlank, Position: pos})
			i++
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			doc.Nodes = append(doc.Nodes, Node{
				Kind:     NodeComment,
				Position: pos,
				Raw:      ln.text,
				Text:     trimmed,
			})
			i++
			continue
		}

		switch {
		case isInclude(trimmed):
			node, err := p.parseInclude(ln.text, pos)
			if err != nil {
				return nil, err
			}
			doc.Nodes = append(doc.Nodes, node)
			i++

		case isKeyword(trimmed, "unset"):
			key := strings.TrimSpace(trimmed[len("unset"):])
			if key == "" {
				return nil, diagnostic.Atf(diagnostic.ErrInvalidSyntax, pos, ln.text, "unset requires a variable name")
			}
			doc.Nodes = append(doc.Nodes, Node{Kind: NodeUnset, Position: pos, Raw: ln.text, Key: key})
			i++

		case isKeyword(trimmed, "required"):
			key := strings.TrimSpace(trimmed[len("required"):])
			if key == "" {
				return nil, diagnostic.Atf(diagnostic.ErrInvalidSyntax, pos, ln.text, "required requires a variable name")
			}
			doc.Nodes = append(doc.Nodes, Node{Kind: NodeRequired, Position: pos, Raw: ln.text, Key: key})
			i++

		case isShellDirective(trimmed):
			return nil, diagnostic.Atf(diagnostic.ErrInvalidSyntax, pos, ln.text,
				"unsupported directive %q: envmerge merges templates and never executes shell commands", firstWord(trimmed))

		default:
			node, consumed, err := p.parseVariable(lines, i, pos)
			if err != nil {
				return nil, err
			}
			doc.Nodes = append(doc.Nodes, node)
			i += consumed
		}
	}

	return doc, nil
}

// parseVariable handles single- and multi-line quoted values, returning the node
// and how many physical lines it consumed.
func (p *parser) parseVariable(lines []physicalLine, start int, pos diagnostic.Position) (Node, int, error) {
	ln := lines[start].text
	eq := strings.IndexByte(ln, '=')
	if eq < 0 {
		return Node{}, 0, diagnostic.Atf(diagnostic.ErrInvalidSyntax, pos, ln, "expected KEY=VALUE but found no '=' separator")
	}
	key := strings.TrimSpace(ln[:eq])
	if !validKey(key) {
		return Node{}, 0, diagnostic.Atf(diagnostic.ErrInvalidVariable, pos, ln, "invalid variable name %q", key)
	}

	rest := strings.TrimLeft(ln[eq+1:], " \t")
	consumed := 1

	node := Node{Kind: NodeVariable, Position: pos, Key: key}

	if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
		quote := rest[0]
		acc := rest
		quoted, remainder, ok := extractQuoted(quote, acc)
		for !ok && start+consumed < len(lines) {
			acc += "\n" + lines[start+consumed].text
			consumed++
			quoted, remainder, ok = extractQuoted(quote, acc)
		}
		if !ok {
			return Node{}, 0, diagnostic.Atf(diagnostic.ErrInvalidSyntax, pos, ln, "unterminated %c-quoted value", rune(quote))
		}
		node.Value = quoted
		node.InlineComment = parseInline(remainder)
		node.Raw = strings.Join(rawTexts(lines[start:start+consumed]), "\n")
		return node, consumed, nil
	}

	node.Value, node.InlineComment = splitUnquoted(rest)
	node.Raw = ln
	return node, consumed, nil
}

func (p *parser) parseInclude(text string, pos diagnostic.Position) (Node, error) {
	trimmed := strings.TrimSpace(text)
	rest := trimmed[len("include"):]
	optional := false
	if strings.HasPrefix(rest, "?") {
		optional = true
		rest = rest[1:]
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		return Node{}, diagnostic.Atf(diagnostic.ErrInvalidInclude, pos, text, "include requires a path")
	}
	path, err := unquotePath(rest)
	if err != nil {
		return Node{}, diagnostic.Atf(diagnostic.ErrInvalidInclude, pos, text, "%s", err)
	}
	return Node{
		Kind:        NodeInclude,
		Position:    pos,
		Raw:         text,
		IncludePath: path,
		Optional:    optional,
	}, nil
}
