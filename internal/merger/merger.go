package merger

import (
	"sort"

	"github.com/soyacen/envmerge/internal/diagnostic"
	"github.com/soyacen/envmerge/internal/parser"
	"github.com/soyacen/envmerge/internal/resolver"
)

// Merger collapses a resolved tree into an ordered variable set.
type Merger interface {
	Merge(document *resolver.ResolvedDocument, policy ConflictPolicy) (*MergedDocument, error)
}

// Engine is the concrete merger.
type Engine struct{}

// New returns a merger engine.
func New() *Engine { return &Engine{} }

type traversal struct {
	om        *orderedMap
	seq       int
	unsetKeys map[string]diagnostic.Position
	required  []string
}

// Merge walks the tree in deterministic order and applies the conflict policy.
func (e *Engine) Merge(document *resolver.ResolvedDocument, policy ConflictPolicy) (*MergedDocument, error) {
	t := &traversal{om: newOrderedMap(), unsetKeys: map[string]diagnostic.Position{}}
	t.walk(document)

	m, err := t.build(policy)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// walk performs a pre-order traversal visiting includes before a document's own
// declarations, assigning an increasing sequence number to every variable and
// marking the first own variable of each document as a group start.
func (t *traversal) walk(rd *resolver.ResolvedDocument) {
	for _, child := range rd.Includes {
		t.walk(child)
	}

	firstOwn := true
	for _, node := range rd.Document.Nodes {
		switch node.Kind {
		case parser.NodeVariable:
			t.om.add(occurrence{
				key:         node.Key,
				value:       node.Value,
				inline:      node.InlineComment,
				leading:     node.LeadingComments,
				blankBefore: node.BlankBefore,
				groupStart:  firstOwn,
				depth:       rd.Depth,
				seq:         t.seq,
				pos:         node.Position,
			})
			if firstOwn {
				firstOwn = false
			}
			t.seq++
		case parser.NodeUnset:
			t.unsetKeys[node.Key] = node.Position
		case parser.NodeRequired:
			t.required = append(t.required, node.Key)
		}
	}
}

// build selects the winning occurrence per key and assembles the merged doc.
func (t *traversal) build(policy ConflictPolicy) (*MergedDocument, error) {
	out := &MergedDocument{Required: dedupe(t.required)}

	// Stable output order is by first appearance (minimum seq per key).
	order := t.om.keys
	sort.SliceStable(order, func(i, j int) bool {
		return firstSeq(t.om.byKey[order[i]]) < firstSeq(t.om.byKey[order[j]])
	})

	for _, key := range order {
		occ := t.om.byKey[key]
		first := occ[0]

		if policy == ConflictError && len(occ) > 1 {
			return nil, diagnostic.Atf(diagnostic.ErrDuplicateVariable, occ[1].pos, occ[1].key,
				"duplicate variable %q (also declared at %s)", key, occ[0].pos.String())
		}

		win := selectWinner(occ, policy)
		v := &Variable{
			Key:             key,
			Value:           win.value,
			InlineComment:   win.inline,
			LeadingComments: first.leading,
			BlankBefore:     first.blankBefore,
			GroupStart:      first.groupStart,
			WinnerPos:       win.pos,
			FirstPos:        first.pos,
		}
		// Record an override whenever the winning declaration is not the first.
		if win.seq != first.seq {
			out.Overrides = append(out.Overrides, Override{Key: key, Previous: first.pos, Current: win.pos})
		}
		// Detect empty winner of an empty literal is fine; just append.
		if _, removed := t.unsetKeys[key]; !removed {
			out.Variables = append(out.Variables, v)
		}
	}

	out.Unset = unsetKeysSorted(t.unsetKeys)

	// Required validation: every required key must survive merge and be non-empty.
	present := map[string]*Variable{}
	for _, v := range out.Variables {
		present[v.Key] = v
	}
	for _, req := range out.Required {
		v, ok := present[req]
		if !ok {
			return nil, diagnostic.New(diagnostic.ErrRequiredMissing, "required variable is missing: "+req)
		}
		if isEmptyValue(v.Value) {
			return nil, diagnostic.New(diagnostic.ErrRequiredMissing, "required variable has an empty value: "+req)
		}
	}
	return out, nil
}

// selectWinner returns the occurrence that wins under the policy.
func selectWinner(occ []occurrence, policy ConflictPolicy) occurrence {
	best := occ[0]
	for _, c := range occ[1:] {
		if beats(c, best, policy) {
			best = c
		}
	}
	return best
}

func beats(a, b occurrence, policy ConflictPolicy) bool {
	switch policy {
	case ConflictLastWins:
		return a.seq > b.seq
	case ConflictFirstWins:
		return false
	case ConflictLocalWins:
		if a.depth != b.depth {
			return a.depth < b.depth // closer to entry wins
		}
		return a.seq > b.seq // later declaration wins at equal depth
	default: // ConflictError: winner is irrelevant, first kept for shape
		return false
	}
}

func firstSeq(occ []occurrence) int {
	m := occ[0].seq
	for _, o := range occ[1:] {
		if o.seq < m {
			m = o.seq
		}
	}
	return m
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func unsetKeysSorted(m map[string]diagnostic.Position) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isEmptyValue(v string) bool {
	s := v
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	}
	return s == ""
}
