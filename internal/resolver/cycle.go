package resolver

import (
	"path/filepath"

	"github.com/soyacen/envmerge/internal/diagnostic"
	"github.com/soyacen/envmerge/internal/parser"
)

// cycleError renders the circular path from the first frame that introduced the
// repeated file through the include that closed the loop, e.g.
//
//	a.env.example -> b.env.example -> c.env.example -> a.env.example
func (s *session) cycleError(id string, startIdx int, closing *parser.Node) error {
	path := make([]string, 0, len(s.activeOrder)-startIdx+1)
	for i := startIdx; i < len(s.activeOrder); i++ {
		path = append(path, filepath.Base(s.activeOrder[i]))
	}
	path = append(path, filepath.Base(id))

	head := diagnostic.New(diagnostic.ErrCircularInclude, path[0])
	prev := head
	for _, seg := range path[1:] {
		node := diagnostic.New(diagnostic.ErrCircularInclude, seg)
		prev.Previous = node
		prev = node
	}
	if closing != nil {
		head.Snippet = closing.Raw
		head.Pos = closing.Position
	}
	return head
}

// contextualInclude annotates a file-not-found error raised while following an
// include, attaching the include directive's source position for diagnostics.
func contextualInclude(err error, node *parser.Node) error {
	if de, ok := err.(*diagnostic.Error); ok {
		if de.Snippet == "" {
			de.Snippet = node.Raw
			de.Pos = node.Position
		}
		return de
	}
	return err
}
