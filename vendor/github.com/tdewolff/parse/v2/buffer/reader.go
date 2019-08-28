package buffer

import "io"

// Reader implements an io.Reader over a byte slice.
type Reader struct {
	buf []byte
	pos int
}

// NewReader returns a new Reader for a given byte slice.
func NewReader(buf []byte) *Reader {
	return &Reader{
		buf: buf,
	}
}

// Read reads bytes into the given byte slice and returns the number of bytes read and an error if occurred.
func (r *Reader) Read(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	n = copy(b, r.buf[r.pos:])
	r.pos += n
	return
}

// Bytes returns the underlying byte slice.
func (r *Reader) Bytes() []byte {
	return r.buf
}

// Reset resets the position of the read pointer to the beginning of the underlying byte slice.
func (r *Reader) Reset() {
	r.pos = 0
}

// Len returns the length of the buffer.
func (r *Reader) Len() int {
	return len(r.buf)
}
