package parser

import (
	"testing"
)

func TestParseError_WithLineCol(t *testing.T) {
	err := &ParseError{
		Line:    5,
		Col:     12,
		Message: "syntax error at or near 'INVALID'",
	}
	want := "line 5:12: syntax error at or near 'INVALID'"
	if got := err.Error(); got != want {
		t.Errorf("ParseError.Error() = %q, want %q", got, want)
	}
}

func TestParseError_WithoutLineCol(t *testing.T) {
	err := &ParseError{
		Message: "duplicate table name: \"users\"",
	}
	want := "duplicate table name: \"users\""
	if got := err.Error(); got != want {
		t.Errorf("ParseError.Error() = %q, want %q", got, want)
	}
}

func TestParseError_Unwrap(t *testing.T) {
	inner := &ParseError{Message: "inner error"}
	outer := &ParseError{Message: "outer", Err: inner}
	if got := outer.Unwrap(); got != inner {
		t.Errorf("ParseError.Unwrap() = %v, want %v", got, inner)
	}
}
