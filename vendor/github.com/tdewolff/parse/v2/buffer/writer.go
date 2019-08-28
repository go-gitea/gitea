package buffer

// Writer implements an io.Writer over a byte slice.
type Writer struct {
	buf []byte
}

// NewWriter returns a new Writer for a given byte slice.
func NewWriter(buf []byte) *Writer {
	return &Writer{
		buf: buf,
	}
}

// Write writes bytes from the given byte slice and returns the number of bytes written and an error if occurred. When err != nil, n == 0.
func (w *Writer) Write(b []byte) (int, error) {
	n := len(b)
	end := len(w.buf)
	if end+n > cap(w.buf) {
		buf := make([]byte, end, 2*cap(w.buf)+n)
		copy(buf, w.buf)
		w.buf = buf
	}
	w.buf = w.buf[:end+n]
	return copy(w.buf[end:], b), nil
}

// Len returns the length of the underlying byte slice.
func (w *Writer) Len() int {
	return len(w.buf)
}

// Bytes returns the underlying byte slice.
func (w *Writer) Bytes() []byte {
	return w.buf
}

// Reset empties and reuses the current buffer. Subsequent writes will overwrite the buffer, so any reference to the underlying slice is invalidated after this call.
func (w *Writer) Reset() {
	w.buf = w.buf[:0]
}
