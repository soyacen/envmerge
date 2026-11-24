package parser

import (
	"errors"
	"strings"
	"unicode"
)

// splitLines splits already-normalized ("\n"-only) text into physical lines,
// dropping the trailing newline and assigning 1-based line numbers.
func splitLines(s string) []physicalLine {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	raw := strings.Split(s, "\n")
	out := make([]physicalLine, len(raw))
	for i, r := range raw {
		out[i] = physicalLine{num: i + 1, text: r}
	}
	return out
}

func rawTexts(lines []physicalLine) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.text
	}
	return out
}

// validKey reports whether s is a legal POSIX-style environment variable name.
func validKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' {
			continue
		}
		if unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return false
	}
	return !unicode.IsDigit(rune(s[0]))
}

func indentWidth(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
		} else if r == '\t' {
			n++
		} else {
			break
		}
	}
	return n
}

func commentBody(trimmed string) string {
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
}

func firstWord(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// isInclude reports a line whose first token is the include keyword. A bare
// "include" (or "include?") is also recognised so its empty argument is
// reported as ERR_INVALID_INCLUDE rather than a generic syntax error.
func isInclude(trimmed string) bool {
	if trimmed == "include" || trimmed == "include?" {
		return true
	}
	if !strings.HasPrefix(trimmed, "include") {
		return false
	}
	rest := trimmed[len("include"):]
	if rest == "" {
		return false
	}
	return rest[0] == ' ' || rest[0] == '\t' || rest[0] == '?'
}

// isKeyword reports a line whose first token equals the given directive.
func isKeyword(trimmed, kw string) bool {
	if !strings.HasPrefix(trimmed, kw) {
		return false
	}
	rest := trimmed[len(kw):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// isShellDirective rejects sourcing shorthand that envmerge must never run.
func isShellDirective(trimmed string) bool {
	if isKeyword(trimmed, "source") {
		return true
	}
	return strings.HasPrefix(trimmed, ". ") || trimmed == "."
}

// splitUnquoted separates an unquoted value from a trailing " #comment". A '#'
// with no preceding whitespace belongs to the value (e.g. abc#123).
func splitUnquoted(s string) (value, inline string) {
	s = strings.TrimRight(s, " \t")
	for i := 1; i < len(s); i++ {
		if s[i] == '#' && (s[i-1] == ' ' || s[i-1] == '\t') {
			value = strings.TrimRight(s[:i], " \t")
			inline = commentBody(strings.TrimSpace(s[i:]))
			return value, inline
		}
	}
	return s, ""
}

// extractQuoted scans a quoted region beginning at s[0] (which must be the
// quote char). It returns the quoted text including both quotes and the
// remainder after the closing quote. For double quotes a backslash escapes the
// next character; single quotes have no escaping.
func extractQuoted(quote byte, s string) (quoted, remainder string, ok bool) {
	if len(s) == 0 || s[0] != quote {
		return "", "", false
	}
	for i := 1; i < len(s); i++ {
		if quote == '"' && s[i] == '\\' {
			i++ // skip escaped character
			continue
		}
		if s[i] == quote {
			return s[:i+1], s[i+1:], true
		}
	}
	return "", "", false
}

// parseInline converts text following a closing quote into an inline comment.
func parseInline(remainder string) string {
	trimmed := strings.TrimSpace(remainder)
	if strings.HasPrefix(trimmed, "#") {
		return commentBody(trimmed)
	}
	return ""
}

// unquotePath parses the argument of an include directive, accepting an
// optionally single- or double-quoted path (which may contain spaces).
func unquotePath(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("include path is empty")
	}
	if q := s[0]; q == '"' || q == '\'' {
		if len(s) < 2 || s[len(s)-1] != q {
			return "", errors.New("unterminated quoted include path")
		}
		inner := s[1 : len(s)-1]
		if inner == "" {
			return "", errors.New("include path is empty")
		}
		return inner, nil
	}
	// Unquoted path: take the first whitespace-delimited token.
	if fields := strings.Fields(s); len(fields) > 0 {
		return fields[0], nil
	}
	return "", errors.New("include path is empty")
}
