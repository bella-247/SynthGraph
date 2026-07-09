package postgresql

import (
	"errors"
	"strings"
	"testing"

	"synthgraph/internal/parser"
)

func TestParse_ReturnsParseError_ForSyntaxError(t *testing.T) {
	invalidSQL := []byte("CREATE INVALID")
	_, err := New().Parse(invalidSQL)
	if err == nil {
		t.Fatal("expected error for invalid SQL, got nil")
	}
	var parseErr *parser.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *parser.ParseError, got %T: %v", err, err)
	}
	if parseErr.Message == "" {
		t.Error("ParseError.Message should not be empty")
	}
}

func TestParse_ReturnsParseError_ForDuplicateColumn(t *testing.T) {
	invalidSQL := []byte("CREATE TABLE t (id INT, id INT);")
	_, err := New().Parse(invalidSQL)
	if err == nil {
		t.Fatal("expected error for duplicate column, got nil")
	}
	var parseErr *parser.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *parser.ParseError, got %T: %v", err, err)
	}
	if parseErr.Message == "" {
		t.Error("ParseError.Message should not be empty")
	}
}

func TestParse_PositionInSyntaxError(t *testing.T) {
	// pg_query reports "at or near 'INVALID'" — we search for "INVALID"
	// in the original source to determine line:col.
	invalidSQL := []byte("CREATE INVALID")
	_, err := New().Parse(invalidSQL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var parseErr *parser.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *parser.ParseError, got %T", err)
	}
	if parseErr.Line != 1 {
		t.Errorf("expected Line=1, got %d", parseErr.Line)
	}
	if parseErr.Col != 8 {
		t.Errorf("expected Col=8 (start of INVALID), got %d", parseErr.Col)
	}
}

func TestParse_PositionOnLaterLine(t *testing.T) {
	// Error is on line 3, column 8 (start of "INVALID")
	sql := []byte("CREATE TABLE t (\n  id INT\n);\nCREATE INVALID")
	_, err := New().Parse(sql)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var parseErr *parser.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *parser.ParseError, got %T", err)
	}
	if parseErr.Line != 4 {
		t.Errorf("expected Line=4, got %d", parseErr.Line)
	}
	if parseErr.Col != 8 {
		t.Errorf("expected Col=8 (start of INVALID), got %d", parseErr.Col)
	}
}

func TestParse_CleanedMessage(t *testing.T) {
	_, err := New().Parse([]byte("CREATE INVALID"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pe *parser.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *parser.ParseError, got %T", err)
	}
	if strings.Contains(pe.Message, "postgresql") {
		t.Errorf("ParseError.Message should not contain internal prefix, got: %q", pe.Message)
	}
}

func TestParse_CleanedMessage_DuplicateColumn(t *testing.T) {
	_, err := New().Parse([]byte("CREATE TABLE t (id INT, id INT);"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pe *parser.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *parser.ParseError, got %T", err)
	}
	if strings.Contains(pe.Message, "postgresql") {
		t.Errorf("ParseError.Message should not contain internal prefix, got: %q", pe.Message)
	}
}

func TestParse_PositionWithComment(t *testing.T) {
	// Token search finds "INVALID" on the correct line even with a comment above
	sql := []byte("-- comment line\nCREATE INVALID")
	_, err := New().Parse(sql)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var parseErr *parser.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *parser.ParseError, got %T", err)
	}
	if parseErr.Line != 2 {
		t.Errorf("expected Line=2, got %d", parseErr.Line)
	}
	if parseErr.Col != 8 {
		t.Errorf("expected Col=8 (start of INVALID), got %d", parseErr.Col)
	}
}
