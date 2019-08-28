package parse

import (
	"fmt"
	"io"

	"github.com/tdewolff/parse/v2/buffer"
)

// Error is a parsing error returned by parser. It contains a message and an offset at which the error occurred.
type Error struct {
	Message string
	r       io.Reader
	Offset  int
	line    int
	column  int
	context string
}

// NewError creates a new error
func NewError(msg string, r io.Reader, offset int) *Error {
	return &Error{
		Message: msg,
		r:       r,
		Offset:  offset,
	}
}

// NewErrorLexer creates a new error from an active Lexer.
func NewErrorLexer(msg string, l *buffer.Lexer) *Error {
	r := buffer.NewReader(l.Bytes())
	offset := l.Offset()
	return NewError(msg, r, offset)
}

// Positions re-parses the file to determine the line, column, and context of the error.
// Context is the entire line at which the error occurred.
func (e *Error) Position() (int, int, string) {
	if e.line == 0 {
		e.line, e.column, e.context = Position(e.r, e.Offset)
	}
	return e.line, e.column, e.context
}

// Error returns the error string, containing the context and line + column number.
func (e *Error) Error() string {
	line, column, context := e.Position()
	return fmt.Sprintf("parse error:%d:%d: %s\n%s", line, column, e.Message, context)
}
