// Package merger collapses a resolved include tree into a single deterministic
// output stream of comment, blank-line and variable blocks. It implements the
// default proximity ("就近原则") strategy plus last-wins / first-wins / error
// alternatives, handles unset and required directives, relocates losing
// declarations' comments onto the surviving assignment, and records overrides
// for diagnostics.
package merger

import "github.com/soyacen/envmerge/internal/diagnostic"

// ConflictPolicy selects how duplicate variable declarations are resolved.
type ConflictPolicy string

const (
	// ConflictLocalWins resolves by proximity: a declaration closer to the
	// entry (smaller include depth) wins; ties at equal depth are broken by
	// later declaration order; within one file later lines win.
	ConflictLocalWins ConflictPolicy = "local-wins"
	ConflictLastWins  ConflictPolicy = "last-wins"
	ConflictFirstWins ConflictPolicy = "first-wins"
	ConflictError     ConflictPolicy = "error"
)

// ParseConflictPolicy validates a policy name.
func ParseConflictPolicy(s string) (ConflictPolicy, bool) {
	switch ConflictPolicy(s) {
	case ConflictLocalWins, ConflictLastWins, ConflictFirstWins, ConflictError:
		return ConflictPolicy(s), true
	default:
		return "", false
	}
}

// BlockKind selects which field of a Block carries its payload.
type BlockKind uint8

const (
	// BlockComment is one comment line, reproduced verbatim from a template.
	BlockComment BlockKind = iota
	// BlockBlank is a blank-line separator; consecutive blanks collapse.
	BlockBlank
	// BlockVariable is an environment variable assignment.
	BlockVariable
)

// String returns a human-readable block kind name.
func (k BlockKind) String() string {
	switch k {
	case BlockComment:
		return "comment"
	case BlockBlank:
		return "blank"
	case BlockVariable:
		return "variable"
	default:
		return "unknown"
	}
}

// MovedRun is one relocated comment run: the lines written above a declaration
// that lost a conflict, kept as documentation of the replacement. From is where
// those lines were written in the template, so explain and --verbose can point
// at the comment's origin rather than at the swallowed assignment.
type MovedRun struct {
	From  diagnostic.Position
	Lines []string
}

// Block is one unit of the rendered output. The merged document is a flat,
// deterministic stream of blocks, so comments and blank lines are preserved
// wherever they appeared - including comment-only templates, file headers,
// comments above an include directive and dangling comments at end of file.
//
// Blocks are stored in emission order and are never reordered: only a losing
// duplicate's comment run may be *relocated* onto the surviving assignment.
type Block struct {
	Kind BlockKind

	// Text is the comment line for BlockComment, including its leading '#'.
	Text string
	// Pos is the template position a BlockComment was read from.
	Pos diagnostic.Position
	// Var is the variable payload for BlockVariable.
	Var *Variable
	// Moved are the comment runs relocated onto this variable, rendered just
	// above it. Only BlockVariable entries carry it.
	Moved []MovedRun
	// GroupStart marks the first block contributed by a new template file; the
	// renderer guarantees a blank line before it.
	GroupStart bool
}

// Variable is a single merged environment variable: the winning value together
// with the position where the key was first seen (so output order stays stable)
// and the provenance needed by explain/verbose.
type Variable struct {
	Key   string
	Value string
	// InlineComment is the winning declaration's trailing comment (no '#').
	InlineComment string

	// WinnerPos is where the winning value was declared; FirstPos is where the
	// key first appeared.
	WinnerPos diagnostic.Position
	FirstPos  diagnostic.Position
}

// Override records a single value replacement.
type Override struct {
	Key      string
	Previous diagnostic.Position
	Current  diagnostic.Position
}

// MergedDocument is the merger result: the renderable block stream plus the
// variable index and diagnostics.
type MergedDocument struct {
	// Blocks is the full output stream, in emission order.
	Blocks []Block
	// Variables exposes the emitted variables (the same values referenced by
	// the BlockVariable entries of Blocks) in output order.
	Variables []*Variable
	Overrides []Override
	Required  []string
	Unset     []string
}

// occurrence is one declaration of a key, tagged with traversal coordinates.
type occurrence struct {
	key    string
	value  string
	inline string
	depth  int
	seq    int
	pos    diagnostic.Position
	// block indexes the provisional variable block this declaration produced.
	block int
}

// orderedMap preserves first-appearance order of keys.
type orderedMap struct {
	keys  []string
	byKey map[string][]occurrence
}

func newOrderedMap() *orderedMap {
	return &orderedMap{byKey: map[string][]occurrence{}}
}

func (o *orderedMap) add(occ occurrence) {
	list, ok := o.byKey[occ.key]
	if !ok {
		o.keys = append(o.keys, occ.key)
		o.byKey[occ.key] = []occurrence{occ}
		return
	}
	o.byKey[occ.key] = append(list, occ)
}
