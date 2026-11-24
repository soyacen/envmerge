package diagnostic

import (
	"errors"
	"fmt"
	"strings"
)

// Code is a stable machine-readable error identifier documented in the design
// specification (section thirteen). Codes are surfaced in CLI output as
// "Error [CODE]" and used for exit-code mapping.
type Code string

const (
	ErrFileNotFound      Code = "ERR_FILE_NOT_FOUND"
	ErrInvalidSyntax     Code = "ERR_INVALID_SYNTAX"
	ErrInvalidInclude    Code = "ERR_INVALID_INCLUDE"
	ErrCircularInclude   Code = "ERR_CIRCULAR_INCLUDE"
	ErrMaxDepth          Code = "ERR_MAX_DEPTH"
	ErrDuplicateVariable Code = "ERR_DUPLICATE_VARIABLE"
	ErrInvalidVariable   Code = "ERR_INVALID_VARIABLE"
	ErrOutputExists      Code = "ERR_OUTPUT_EXISTS"
	ErrPathOutsideRoot   Code = "ERR_PATH_OUTSIDE_ROOT"
	ErrPermissionDenied  Code = "ERR_PERMISSION_DENIED"
	ErrWriteFailed       Code = "ERR_WRITE_FAILED"
	ErrOutputMismatch    Code = "ERR_OUTPUT_MISMATCH"
	ErrRequiredMissing   Code = "ERR_REQUIRED_MISSING"
)

// Error lets a Code be used directly as an error message.
func (c Code) Error() string {
	if d, ok := desc[c]; ok {
		return string(c) + ": " + d
	}
	return string(c)
}

// desc maps a code to its human-readable description.
var desc = map[Code]string{
	ErrFileNotFound:      "template file does not exist",
	ErrInvalidSyntax:     "invalid template syntax",
	ErrInvalidInclude:    "invalid include directive",
	ErrCircularInclude:   "circular include detected",
	ErrMaxDepth:          "maximum include depth exceeded",
	ErrDuplicateVariable: "duplicate environment variable",
	ErrInvalidVariable:   "invalid environment variable",
	ErrOutputExists:      "output file already exists",
	ErrPathOutsideRoot:   "path is outside the allowed root",
	ErrPermissionDenied:  "insufficient file permissions",
	ErrWriteFailed:       "file write failed",
	ErrOutputMismatch:    "output file does not match the template",
	ErrRequiredMissing:   "required variable is missing or empty",
}

// Error is the structured error type carried across all subsystems. It always
// has a Code, may carry a Position, an offending source line for context, and a
// cause for errors.Is/As chains.
type Error struct {
	Code     Code
	Message  string
	Pos      Position
	Snippet  string
	Details  []string
	Previous *Error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(string(e.Code))
	b.WriteString(": ")
	b.WriteString(e.Message)
	if e.Pos.Valid() {
		b.WriteString(" at ")
		b.WriteString(e.Pos.String())
	}
	for _, d := range e.Details {
		b.WriteString("\n    ")
		b.WriteString(d)
	}
	if e.Previous != nil {
		b.WriteString("\ncircular path:\n  ")
		b.WriteString(e.Previous.pathChain())
	}
	return b.String()
}

// pathChain renders the recorded circular chain as an indented "a -> b -> c".
func (e *Error) pathChain() string {
	var parts []string
	for cur := e; cur != nil; cur = cur.Previous {
		parts = append(parts, cur.Message)
	}
	return strings.Join(parts, "\n  -> ")
}

// Unwrap exposes the previous link for errors.Is traversal.
func (e *Error) Unwrap() error {
	if e.Previous == nil {
		return nil
	}
	return e.Previous
}

// CodeOf returns the diagnostic Code carried by err, or "" when err is not a
// *Error.
func CodeOf(err error) Code {
	var de *Error
	if errors.As(err, &de) {
		return de.Code
	}
	return ""
}

// New constructs a structured error.
func New(code Code, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// Atf constructs a structured error with a position and an optional source
// snippet line for context, using fmt-style formatting for the message.
func Atf(code Code, pos Position, snippet, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Pos:     pos,
		Snippet: snippet,
	}
}
