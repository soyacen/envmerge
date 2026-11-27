package merger

import (
	"sort"
	"strconv"

	"github.com/soyacen/envmerge/internal/diagnostic"
	"github.com/soyacen/envmerge/internal/parser"
	"github.com/soyacen/envmerge/internal/resolver"
)

// Merger collapses a resolved tree into an ordered block stream.
type Merger interface {
	Merge(document *resolver.ResolvedDocument, policy ConflictPolicy) (*MergedDocument, error)
}

// Engine is the concrete merger.
type Engine struct{}

// New returns a merger engine.
func New() *Engine { return &Engine{} }

// traversal builds the provisional block stream in template order, then applies
// the conflict policy to produce the merged document.
type traversal struct {
	om        *orderedMap
	seq       int
	unsetKeys map[string]diagnostic.Position
	required  []string

	blocks []Block
	// varsByBlock indexes the variable allocated for each provisional variable
	// block, so a winning value can be written onto the first occurrence.
	varsByBlock map[int]*Variable
	// emittedComments records comment source positions already rendered, so a
	// template reached through several include paths (a diamond) contributes its
	// comments exactly once.
	emittedComments map[string]bool
}

// Merge walks the tree in deterministic order and applies the conflict policy.
func (e *Engine) Merge(document *resolver.ResolvedDocument, policy ConflictPolicy) (*MergedDocument, error) {
	t := &traversal{
		om:              newOrderedMap(),
		unsetKeys:       map[string]diagnostic.Position{},
		varsByBlock:     map[int]*Variable{},
		emittedComments: map[string]bool{},
	}
	t.walk(document, true)

	m, err := t.build(policy)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// walk expands one document at the position of its own statements. Comments and
// blank lines are recorded verbatim; includes are expanded where they are
// declared, so a comment written above an include stays above the content that
// include contributes, and a template with no variables at all still contributes
// its comments.
func (t *traversal) walk(rd *resolver.ResolvedDocument, entry bool) {
	source := rd.ID
	if source == "" {
		source = rd.AbsPath
	}

	// The entry file starts flush with the top of the output; every nested file
	// begins a new group, separated by a blank line.
	first := !entry

	for _, node := range rd.Document.Nodes {
		switch node.Kind {
		case parser.NodeBlank:
			t.append(Block{Kind: BlockBlank}, first)
			first = false

		case parser.NodeComment:
			id := source + ":" + strconv.Itoa(node.Position.Line)
			if !t.emittedComments[id] {
				t.emittedComments[id] = true
				t.append(Block{Kind: BlockComment, Text: node.Text, Pos: node.Position}, first)
			}
			first = false

		case parser.NodeInclude:
			for _, child := range t.children(rd, node) {
				t.walk(child, false)
			}
			// The parent's next own statement starts a new group again, so a
			// comment contributed by a child cannot read as if it documented
			// the parent's declaration.
			first = true

		case parser.NodeVariable:
			v := &Variable{Key: node.Key, FirstPos: node.Position}
			idx := t.append(Block{Kind: BlockVariable, Var: v}, first)
			first = false
			t.varsByBlock[idx] = v
			t.om.add(occurrence{
				key:    node.Key,
				value:  node.Value,
				inline: node.InlineComment,
				depth:  rd.Depth,
				seq:    t.seq,
				pos:    node.Position,
				block:  idx,
			})
			t.seq++

		case parser.NodeUnset:
			t.unsetKeys[node.Key] = node.Position

		case parser.NodeRequired:
			t.required = append(t.required, node.Key)
		}
	}
}

// append records a block and returns its provisional index. Only comment and
// variable blocks carry the group marker; a blank block is itself a separator.
func (t *traversal) append(b Block, groupStart bool) int {
	if groupStart && b.Kind != BlockBlank {
		b.GroupStart = true
	}
	t.blocks = append(t.blocks, b)
	return len(t.blocks) - 1
}

// children returns the documents produced by one include directive, matched by
// the position of the directive inside the declaring document.
func (t *traversal) children(rd *resolver.ResolvedDocument, node parser.Node) []*resolver.ResolvedDocument {
	var out []*resolver.ResolvedDocument
	for i, ref := range rd.Sources {
		if ref.Node.Position == node.Position {
			out = append(out, rd.Includes[i])
		}
	}
	return out
}

// commentRun is a contiguous run of comment blocks waiting for the statement
// they document.
type commentRun []Block

// build selects the winning declaration per key and assembles the block stream.
func (t *traversal) build(policy ConflictPolicy) (*MergedDocument, error) {
	out := &MergedDocument{Required: dedupe(t.required)}

	// Stable output order is by first appearance (minimum seq per key).
	order := t.om.keys
	sort.SliceStable(order, func(i, j int) bool {
		return firstSeq(t.om.byKey[order[i]]) < firstSeq(t.om.byKey[order[j]])
	})

	unset := make(map[string]bool, len(t.unsetKeys))
	for key := range t.unsetKeys {
		unset[key] = true
	}

	// Pass 1: resolve every key onto the block where it first appeared.
	emit := make(map[string]int, len(order))
	dropped := make(map[string]bool, len(order))
	firstPos := make(map[string]diagnostic.Position, len(order))
	for _, key := range order {
		occ := t.om.byKey[key]
		first := occ[0]
		firstPos[key] = first.pos

		if policy == ConflictError && len(occ) > 1 {
			return nil, diagnostic.Atf(diagnostic.ErrDuplicateVariable, occ[1].pos, occ[1].key,
				"duplicate variable %q (also declared at %s)", key, first.pos.String())
		}

		win := selectWinner(occ, policy)
		v := t.varsByBlock[first.block]
		v.Value = win.value
		v.InlineComment = win.inline
		v.WinnerPos = win.pos

		// Record an override whenever the winning declaration is not the first.
		if win.seq != first.seq {
			out.Overrides = append(out.Overrides, Override{Key: key, Previous: first.pos, Current: win.pos})
		}

		if unset[key] {
			// An unset declaration documents why the key is absent, so its
			// comments stay where they were written; only the assignment goes.
			dropped[key] = true
			continue
		}
		emit[key] = first.block
	}
	out.Unset = unsetKeysSorted(t.unsetKeys)

	// Pass 2: replay the stream. Variables that are not the emitting occurrence
	// disappear; the comment run written above such a losing declaration is
	// relocated onto the surviving assignment instead of being left dangling.
	var run commentRun
	rendered := make(map[int]int, len(order))
	for i, b := range t.blocks {
		switch b.Kind {
		case BlockBlank:
			run = nil
			out.Blocks = append(out.Blocks, b)

		case BlockComment:
			run = append(run, b)
			out.Blocks = append(out.Blocks, b)

		case BlockVariable:
			key := b.Var.Key
			idx, keeps := emit[key]
			switch {
			case keeps && idx == i:
				rendered[i] = len(out.Blocks)
				out.Blocks = append(out.Blocks, b)
				out.Variables = append(out.Variables, b.Var)
				run = nil

			case dropped[key]:
				// Unset: drop the assignment, keep its comments in place.
				run = nil

			case keeps && b.Var.FirstPos == firstPos[key]:
				// The very same declaration re-visited through a second include
				// path (a diamond): it was already rendered, and its comments
				// were deduplicated, so nothing here needs relocating.
				run = nil

			default:
				// Losing duplicate: relocate the comment run written above it
				// onto the surviving assignment, then drop the run and the
				// assignment from the stream.
				if pos, ok := rendered[idx]; ok && len(run) > 0 {
					nb := out.Blocks[pos]
					nb.Moved = relocate(nb.Moved, run)
					out.Blocks[pos] = nb
				}
				out.Blocks = out.Blocks[:len(out.Blocks)-len(run)]
				run = nil
			}
		}
	}

	// Required validation: every required key must survive merge and be non-empty.
	present := make(map[string]*Variable, len(out.Variables))
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

// relocate appends a losing declaration's comment run to the surviving
// assignment: it merges with a run already recorded from the same template
// position and skips lines that are already rendered there, so repeated
// declarations of the same override never stack duplicates.
func relocate(dst []MovedRun, r commentRun) []MovedRun {
	if len(r) == 0 {
		return dst
	}
	from := r[0].Pos
	var lines []string
	for _, b := range r {
		if runContains(dst, b.Text) || listContains(lines, b.Text) {
			continue
		}
		lines = append(lines, b.Text)
	}
	if len(lines) == 0 {
		return dst
	}
	for i := range dst {
		if dst[i].From == from {
			dst[i].Lines = append(dst[i].Lines, lines...)
			return dst
		}
	}
	return append(dst, MovedRun{From: from, Lines: lines})
}

func runContains(dst []MovedRun, line string) bool {
	for _, m := range dst {
		if listContains(m.Lines, line) {
			return true
		}
	}
	return false
}

func listContains(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
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
