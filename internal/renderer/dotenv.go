// Package renderer serializes a merged block stream back into dotenv text.
// Comments and blank lines are blocks of their own, so the result reads like a
// human authored file: everything a template documents survives into the
// generated file, and repeated runs are byte-identical.
package renderer

import (
	"strings"

	"github.com/soyacen/envmerge/internal/merger"
)

// Renderer produces dotenv bytes from a merged document.
type Renderer interface {
	Render(document *merger.MergedDocument) ([]byte, error)
}

// Dotenv is the standard dotenv renderer.
type Dotenv struct{}

// New returns a Dotenv renderer.
func New() *Dotenv { return &Dotenv{} }

// Render walks the block stream in emission order. A blank line is emitted for
// every blank block and before the first block of a new template file; runs of
// blank lines collapse into one and the file never starts or ends with a blank
// line. Comment text is reproduced verbatim and variable values exactly as
// declared, quotes included. Relocated comment runs are rendered directly above
// the assignment that absorbed them; no provenance text is invented - the source
// of an override is reported by `explain` and `--verbose`.
func (r *Dotenv) Render(document *merger.MergedDocument) ([]byte, error) {
	var (
		lines    []string
		wrote    bool
		needsGap bool
	)

	// gap flushes the pending separation, if any content exists to separate.
	gap := func() {
		if needsGap && wrote {
			lines = append(lines, "")
		}
		needsGap = false
		wrote = true
	}

	for _, b := range document.Blocks {
		switch b.Kind {
		case merger.BlockBlank:
			needsGap = true

		case merger.BlockComment:
			needsGap = needsGap || b.GroupStart
			gap()
			lines = append(lines, b.Text)

		case merger.BlockVariable:
			needsGap = needsGap || b.GroupStart
			gap()
			for _, moved := range b.Moved {
				lines = append(lines, moved.Lines...)
			}
			v := b.Var
			line := v.Key + "=" + v.Value
			if v.InlineComment != "" {
				line += " # " + v.InlineComment
			}
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return nil, nil
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}
