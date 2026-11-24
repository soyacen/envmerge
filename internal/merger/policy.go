// Package merger collapses a resolved include tree into a single ordered set of
// environment variables. It implements the default proximity ("就近原则")
// strategy plus last-wins / first-wins / error alternatives, handles unset and
// required directives, and records overrides for diagnostics.
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

// Variable is a single merged environment variable, carrying both the winning
// value and the positional metadata of its first appearance (so output order and
// comment grouping stay stable and deterministic).
type Variable struct {
	Key   string
	Value string
	// InlineComment is the winning occurrence's trailing comment (no leading '#').
	InlineComment string

	// Rendering metadata, taken from the first occurrence.
	LeadingComments []string
	BlankBefore     bool
	GroupStart      bool

	// WinnerPos is where the winning value was declared; FirstPos is where the
	// key first appeared. Both feed explain/verbose output.
	WinnerPos diagnostic.Position
	FirstPos  diagnostic.Position
}

// Override records a single value replacement.
type Override struct {
	Key      string
	Previous diagnostic.Position
	Current  diagnostic.Position
}

// MergedDocument is the merger result.
type MergedDocument struct {
	Variables []*Variable
	Overrides []Override
	Required  []string
	Unset     []string
}

// occurrence is one declaration of a key, tagged with traversal coordinates.
type occurrence struct {
	key         string
	value       string
	inline      string
	leading     []string
	blankBefore bool
	groupStart  bool
	depth, seq  int
	pos         diagnostic.Position
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
