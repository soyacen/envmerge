// Package diagnostic provides source positions and the structured error model
// shared by every envmerge subsystem (parser, resolver, merger, writer).
package diagnostic

// Position records the location of a node inside a template file. It is used
// for deterministic diagnostics and for explaining variable provenance.
type Position struct {
	File   string
	Line   int
	Column int
}

// String renders the position in the canonical "file:line:column" form used by
// the CLI diagnostics. A zero value renders as an empty string.
func (p Position) String() string {
	if p == (Position{}) {
		return ""
	}
	s := p.File
	if p.Line > 0 {
		s += ":" + itoa(p.Line)
		if p.Column > 0 {
			s += ":" + itoa(p.Column)
		}
	}
	return s
}

// Valid reports whether the position carries at least a file reference.
func (p Position) Valid() bool {
	return p.File != ""
}

// itoa is a tiny int-to-string helper avoiding an fmt import on a hot path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
