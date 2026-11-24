// Package resolver expands the include graph of a template into a tree,
// assigning each document an include depth. It detects circular references,
// honors shared (diamond) dependencies without re-reading files, supports
// glob and optional includes, and enforces the root containment policy.
package resolver

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/soyacen/envmerge/internal/diagnostic"
	"github.com/soyacen/envmerge/internal/loader"
	"github.com/soyacen/envmerge/internal/parser"
)

// ResolvedDocument is a template document together with its resolved children,
// in declaration order. The tree shape is preserved so that the merger can apply
// proximity ("就近") semantics per layer.
type ResolvedDocument struct {
	Document *parser.Document
	AbsPath  string
	ID       string
	Depth    int

	Includes []*ResolvedDocument // resolved children in declaration order
	Sources  []*IncludeRef       // the include directives that produced children
}

// IncludeRef records an include node together with the child it produced, used
// for the tree command and circular-path reporting.
type IncludeRef struct {
	Node  parser.Node
	Child *ResolvedDocument
}

// Resolver walks include references recursively.
type Resolver interface {
	Resolve(ctx context.Context, inputPath string) (*ResolvedDocument, error)
}

// Engine is the concrete resolver.
type Engine struct {
	Loader   loader.Loader
	MaxDepth int
	Root     string // optional containment root; "" disables the check
}

// New constructs a resolver engine.
func New(l loader.Loader, maxDepth int, root string) *Engine {
	return &Engine{Loader: l, MaxDepth: maxDepth, Root: root}
}

type session struct {
	e           *Engine
	cache       map[string]*ResolvedDocument
	active      map[string]int // id -> index in activeOrder
	activeOrder []string
	activeNodes []*parser.Node // include node that led to each active frame
}

// Resolve expands inputPath (the entry template) into a tree.
func (e *Engine) Resolve(ctx context.Context, inputPath string) (*ResolvedDocument, error) {
	s := &session{
		e:      e,
		cache:  map[string]*ResolvedDocument{},
		active: map[string]int{},
	}
	return s.visit(ctx, inputPath, 0, nil)
}

func (s *session) visit(ctx context.Context, path string, depth int, from *parser.Node) (*ResolvedDocument, error) {
	if s.e.MaxDepth > 0 && depth > s.e.MaxDepth {
		return nil, diagnostic.New(diagnostic.ErrMaxDepth, "maximum include depth exceeded at "+path)
	}

	abs := filepath.Clean(path)
	id := loader.CanonicalID(abs)
	if err := loader.CheckRoot(s.e.Root, id); err != nil {
		return nil, err
	}

	// Cycle detection must precede the cache: a file already on the active
	// stack is a circular reference even though it is also cached (as the
	// entry or an ancestor). Returning it from cache would build a cyclic tree.
	if idx, cyc := s.active[id]; cyc {
		return nil, s.cycleError(id, idx, from)
	}
	if cached, ok := s.cache[id]; ok {
		return cached, nil
	}

	data, err := s.e.Loader.Load(ctx, abs)
	if err != nil {
		if from != nil {
			return nil, contextualInclude(err, from)
		}
		return nil, err
	}
	doc, err := parser.Parse(abs, data)
	if err != nil {
		return nil, err
	}

	rd := &ResolvedDocument{Document: doc, AbsPath: abs, ID: id, Depth: depth}
	s.cache[id] = rd

	frame := len(s.activeOrder)
	s.active[id] = frame
	s.activeOrder = append(s.activeOrder, id)
	s.activeNodes = append(s.activeNodes, from)

	for i := range doc.Nodes {
		node := &doc.Nodes[i]
		if node.Kind != parser.NodeInclude {
			continue
		}
		children, err := s.expandInclude(ctx, node, abs, depth)
		if err != nil {
			s.pop(frame)
			return nil, err
		}
		for _, child := range children {
			rd.Includes = append(rd.Includes, child)
			rd.Sources = append(rd.Sources, &IncludeRef{Node: *node, Child: child})
		}
	}

	s.pop(frame)
	return rd, nil
}

func (s *session) pop(frame int) {
	if frame < len(s.activeOrder) {
		delete(s.active, s.activeOrder[frame])
		s.activeOrder = s.activeOrder[:frame]
		s.activeNodes = s.activeNodes[:frame]
	}
}

// expandInclude resolves one include directive to zero or more children,
// handling the optional marker, glob patterns and relative-path base rules.
func (s *session) expandInclude(ctx context.Context, node *parser.Node, declaringFile string, depth int) ([]*ResolvedDocument, error) {
	isGlob := strings.ContainsAny(node.IncludePath, "*?[")
	resolved := loader.JoinRelative(declaringFile, node.IncludePath)

	var targets []string
	if isGlob {
		matches, err := s.e.glob(resolved)
		if err != nil {
			return nil, contextualInclude(err, node)
		}
		targets = matches
	} else {
		targets = []string{resolved}
	}

	var out []*ResolvedDocument
	for _, target := range targets {
		child, err := s.visit(ctx, target, depth+1, node)
		if err != nil {
			// Optional includes tolerate missing files / empty matches.
			if node.Optional {
				code := diagnostic.CodeOf(err)
				if code == diagnostic.ErrFileNotFound {
					continue
				}
			}
			return nil, err
		}
		out = append(out, child)
	}
	return out, nil
}

// glob expands a pattern against the OS directory listing, sorted for
// deterministic output (design §18).
func (e *Engine) glob(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, diagnostic.New(diagnostic.ErrInvalidInclude, "invalid include glob: "+pattern)
	}
	sort.Strings(matches)
	return matches, nil
}
