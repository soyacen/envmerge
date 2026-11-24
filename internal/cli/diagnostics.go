package cli

import (
	"errors"
	"fmt"

	"github.com/soyacen/envmerge/internal/diagnostic"
)

// diagnosticCode returns the structured error code carried by err, or "".
func diagnosticCode(err error) diagnostic.Code {
	var de *diagnostic.Error
	if errors.As(err, &de) {
		return de.Code
	}
	return ""
}

// printDiagnostic renders a structured error in the format defined in the
// design specification (section thirteen).
func printDiagnostic(err error) {
	var de *diagnostic.Error
	if !errors.As(err, &de) {
		fmt.Fprintln(stderr, "Error: "+err.Error())
		return
	}

	fmt.Fprintf(stderr, "Error [%s]\n", de.Code)
	if de.Pos.Valid() {
		fmt.Fprintf(stderr, "\n%s\n", de.Pos.String())
	}
	if de.Snippet != "" {
		fmt.Fprintf(stderr, "\n\t%s\n", de.Snippet)
	}
	fmt.Fprintf(stderr, "\n%s\n", de.Message)
	for _, d := range de.Details {
		fmt.Fprintf(stderr, "\t%s\n", d)
	}

	// Circular errors carry their path as a linked list in Previous.
	if de.Code == diagnostic.ErrCircularInclude {
		var chain []string
		for cur := de; cur != nil; cur = cur.Previous {
			chain = append(chain, cur.Message)
		}
		fmt.Fprintln(stderr, "\ncircular path:")
		for i, seg := range chain {
			if i == 0 {
				fmt.Fprintf(stderr, "  %s\n", seg)
			} else {
				fmt.Fprintf(stderr, "  -> %s\n", seg)
			}
		}
	}
}
