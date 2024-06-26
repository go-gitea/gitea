package transfer

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Status codes.
const (
	StatusContinue            uint32 = http.StatusContinue
	StatusOK                  uint32 = http.StatusOK
	StatusCreated             uint32 = http.StatusCreated
	StatusAccepted            uint32 = http.StatusAccepted
	StatusBadRequest          uint32 = http.StatusBadRequest
	StatusForbidden           uint32 = http.StatusForbidden
	StatusNotFound            uint32 = http.StatusNotFound
	StatusMethodNotAllowed    uint32 = http.StatusMethodNotAllowed
	StatusConflict            uint32 = http.StatusConflict
	StatusInternalServerError uint32 = http.StatusInternalServerError
	StatusUnauthorized        uint32 = http.StatusUnauthorized
)

// StatusString returns the status string lowercased for a status code.
func StatusText(code uint32) string {
	return strings.ToLower(http.StatusText(int(code)))
}

// Status is a Git LFS status.
type Status interface {
	Code() uint32
	Args() []string
	Messages() []string
	Reader() io.Reader
}

type status struct {
	code     uint32
	args     []string
	messages []string
	reader   io.Reader
}

// String returns the string representation of the status.
func (s status) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "status %d ", s.code)
	fmt.Fprintf(&b, "args %v ", s.args)
	fmt.Fprintf(&b, "messages %v ", s.messages)
	if s.reader != nil {
		fmt.Fprintf(&b, "reader %v ", s.reader)
	}
	return b.String()
}

// Code returns the status code.
func (s *status) Code() uint32 {
	return s.code
}

// Args returns the status args.
func (s *status) Args() []string {
	return s.args
}

// Messages returns the status messages.
func (s *status) Messages() []string {
	return s.messages
}

// Reader returns the status reader.
func (s *status) Reader() io.Reader {
	return s.reader
}

// SuccessStatus returns a successful status.
func SuccessStatus() Status {
	return NewSuccessStatus()
}

// NewSuccessStatus returns a new successful status.
func NewSuccessStatus(messages ...string) Status {
	return NewSuccessStatusWithArgs(messages)
}

// NewSuccessStatusWithArgs returns a new successful status with data.
func NewSuccessStatusWithArgs(messages []string, args ...string) Status {
	return NewStatusWithArgs(StatusOK, messages, args...)
}

// NewStatus returns a new status with messages.
func NewStatus(code uint32, messages ...string) Status {
	return &status{
		code:     code,
		messages: messages,
	}
}

// NewStatusWithArgs returns a new successful status with data.
func NewStatusWithArgs(code uint32, messages []string, args ...string) Status {
	return &status{
		code:     code,
		args:     args,
		messages: messages,
	}
}

// NewSuccessStatusWithReader returns a new status with a reader.
func NewSuccessStatusWithReader(reader io.Reader, args ...string) Status {
	return &status{
		code:   StatusOK,
		args:   args,
		reader: reader,
	}
}
