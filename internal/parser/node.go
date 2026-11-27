// Package parser turns a raw dotenv template into an AST of nodes that preserve
// comments, blank lines, quoting and source positions. It never interprets the
// value of a variable and never executes shell syntax.
package parser

import "github.com/soyacen/envmerge/internal/diagnostic"

// NodeKind identifies the type of a parsed template line.
type NodeKind uint8

const (
	NodeUnknown NodeKind = iota
	NodeInclude
	NodeVariable
	NodeComment
	NodeBlank
	NodeUnset
	NodeRequired
)

// String returns a human-readable node kind name.
func (k NodeKind) String() string {
	switch k {
	case NodeInclude:
		return "include"
	case NodeVariable:
		return "variable"
	case NodeComment:
		return "comment"
	case NodeBlank:
		return "blank"
	case NodeUnset:
		return "unset"
	case NodeRequired:
		return "required"
	default:
		return "unknown"
	}
}

// Node is a single logical element of a template document: one physical line,
// except for a multi-line quoted variable, which spans every line of its value.
// Unused fields are left at their zero value for the relevant kind.
//
// Nodes are recorded verbatim and in source order. Comments and blank lines are
// first-class nodes; no node claims ownership of the comments around it, so the
// merger can decide where every comment belongs.
type Node struct {
	Kind     NodeKind
	Position diagnostic.Position

	// Raw is the exact source text consumed by this node. For multi-line
	// quoted variables it spans every physical line of the value.
	Raw string

	// Variable / Unset / Required key.
	Key string
	// Value is the text that follows the '=' and is reproduced verbatim on
	// output, quotes included. It is never expanded or re-interpreted.
	Value string
	// InlineComment is a trailing comment (without the leading '#'), if any.
	InlineComment string
	// Text is a full-line comment for NodeComment, whitespace-trimmed and
	// keeping its leading '#', e.g. "# MySQL defaults".
	Text string

	// IncludePath is the (already trimmed) path argument of an include.
	IncludePath string
	// Optional marks an "include?" directive that must not fail when missing.
	Optional bool
}

// Document is the parsed representation of one template file.
type Document struct {
	Path  string
	Nodes []Node
}
