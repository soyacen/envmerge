package diagnostic

import (
	"errors"
	"strings"
	"testing"
)

func TestPositionString(t *testing.T) {
	if (Position{}).String() != "" {
		t.Error("zero position should render empty")
	}
	got := Position{File: "a.env.example", Line: 3, Column: 9}.String()
	if got != "a.env.example:3:9" {
		t.Errorf("got %q", got)
	}
	if (Position{File: "f", Line: 2}).String() != "f:2" {
		t.Error("column omitted when zero")
	}
}

func TestCodeError(t *testing.T) {
	if !strings.HasPrefix(ErrCircularInclude.Error(), "ERR_CIRCULAR_INCLUDE:") {
		t.Errorf("unexpected code error: %q", ErrCircularInclude.Error())
	}
}

func TestErrorChainAndUnwrap(t *testing.T) {
	head := New(ErrCircularInclude, "a.env.example")
	tail := New(ErrCircularInclude, "b.env.example")
	head.Previous = tail
	var inner *Error
	if !errors.As(error(head), &inner) {
		t.Fatal("errors.As failed")
	}
	if CodeOf(head) != ErrCircularInclude {
		t.Error("CodeOf mismatch")
	}
	if !strings.Contains(head.Error(), "b.env.example") {
		t.Errorf("chain should include later segment: %q", head.Error())
	}
	if head.Unwrap() != tail {
		t.Error("Unwrap should return previous link")
	}
}

func TestAtf(t *testing.T) {
	e := Atf(ErrInvalidSyntax, Position{File: "f", Line: 1}, "KEY", "bad %s", "thing")
	if e.Code != ErrInvalidSyntax || e.Snippet != "KEY" || e.Message != "bad thing" {
		t.Errorf("Atf fields wrong: %+v", e)
	}
}
