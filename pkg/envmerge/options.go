// Package envmerge is the public, dependency-light SDK for compiling dotenv
// templates. The same Engine powers the CLI and can be embedded in other Go
// programs. It exposes the core merge pipeline as a reusable library.
package envmerge

import (
	"io/fs"

	"github.com/soyacen/envmerge/internal/loader"
	"github.com/soyacen/envmerge/internal/merger"
)

// ConflictPolicy re-exports the merger policy type for a clean public surface.
type ConflictPolicy = merger.ConflictPolicy

const (
	LocalWins  ConflictPolicy = merger.ConflictLocalWins
	LastWins   ConflictPolicy = merger.ConflictLastWins
	FirstWins  ConflictPolicy = merger.ConflictFirstWins
	ErrorOnDup ConflictPolicy = merger.ConflictError
)

// DefaultMaxIncludeDepth is the recursion ceiling when Options.MaxIncludeDepth
// is zero (design §6).
const DefaultMaxIncludeDepth = 64

// Options configures an Engine. All fields have sensible defaults via apply.
// Comments and blank-line grouping are always preserved: the generated file is
// meant to be read by a human, so there is no mode that throws them away.
type Options struct {
	ConflictPolicy  ConflictPolicy
	MaxIncludeDepth int
	RootDir         string // limits include resolution
	Force           bool   // permit overwriting an existing output

	// Loader overrides file access (used by tests to inject fstest.MapFS).
	Loader loader.Loader
	// FS is a convenience: when set and Loader is nil, an FSLoader over this FS
	// rooted at RootDir is used.
	FS fs.FS
}

// Apply fills zero-value fields with defaults and returns the result.
func (o Options) apply() Options {
	if o.ConflictPolicy == "" {
		o.ConflictPolicy = LocalWins
	}
	if o.MaxIncludeDepth == 0 {
		o.MaxIncludeDepth = DefaultMaxIncludeDepth
	}
	if o.Loader == nil && o.FS != nil {
		root := o.RootDir
		if root == "" {
			root = "."
		}
		o.Loader = &loader.FSLoader{FS: o.FS, Root: root}
	}
	if o.Loader == nil {
		o.Loader = loader.OSLoader{}
	}
	return o
}
