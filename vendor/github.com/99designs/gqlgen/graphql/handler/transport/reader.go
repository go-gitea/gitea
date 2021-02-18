package transport

import (
	"errors"
	"io"
)

type bytesReader struct {
	s        *[]byte
	i        int64 // current reading index
	prevRune int   // index of previous rune; or < 0
}

func (r *bytesReader) Read(b []byte) (n int, err error) {
	if r.s == nil {
		return 0, errors.New("byte slice pointer is nil")
	}
	if r.i >= int64(len(*r.s)) {
		return 0, io.EOF
	}
	r.prevRune = -1
	n = copy(b, (*r.s)[r.i:])
	r.i += int64(n)
	return
}
