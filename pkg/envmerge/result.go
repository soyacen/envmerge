package envmerge

import (
	"github.com/soyacen/envmerge/internal/merger"
	"github.com/soyacen/envmerge/internal/resolver"
)

// Result carries everything a caller needs after compiling a template: the
// rendered bytes, provenance information and the resolved tree for tree/explain
// commands.
type Result struct {
	InputPath  string
	OutputPath string

	Content []byte

	Dependencies []string
	Variables    []*merger.Variable
	Overrides    []merger.Override

	// Merged and Root expose intermediate structures for diagnostics without
	// forcing callers to re-run the pipeline.
	Merged *merger.MergedDocument
	Root   *resolver.ResolvedDocument
}
