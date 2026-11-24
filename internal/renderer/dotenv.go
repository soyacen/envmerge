// Package renderer serializes a merged variable set back into dotenv text,
// preserving comments and blank-line grouping so the result reads like a human
// authored file and stays stable across runs.
package renderer

import (
	"strings"

	"github.com/soyacen/envmerge/internal/merger"
)

// Options control comment and blank-line preservation.
type Options struct {
	PreserveComments   bool
	PreserveBlankLines bool
}

// Renderer produces dotenv bytes from a merged document.
type Renderer interface {
	Render(document *merger.MergedDocument) ([]byte, error)
}

// Dotenv is the standard dotenv renderer.
type Dotenv struct {
	Options Options
}

// New returns a Dotenv renderer.
func New(opts Options) *Dotenv { return &Dotenv{Options: opts} }

// Render writes each variable at its fixed position. A blank line is inserted
// before a variable that begins a new include group or was preceded by a blank
// line in its source (except for the very first line). Comment text is emitted
// verbatim; variable values are reproduced exactly as declared.
func (r *Dotenv) Render(document *merger.MergedDocument) ([]byte, error) {
	var b strings.Builder
	for i, v := range document.Variables {
		if i > 0 && r.Options.PreserveBlankLines && (v.GroupStart || v.BlankBefore) {
			b.WriteString("\n")
		}
		if r.Options.PreserveComments {
			for _, c := range v.LeadingComments {
				b.WriteString(c)
				b.WriteString("\n")
			}
		}
		b.WriteString(v.Key)
		b.WriteString("=")
		b.WriteString(v.Value)
		if r.Options.PreserveComments && v.InlineComment != "" {
			b.WriteString(" # ")
			b.WriteString(v.InlineComment)
		}
		b.WriteString("\n")
	}
	return []byte(b.String()), nil
}
