package ioutil

import (
	"bufio"
	"errors"
	"io"
)

type readPeeker interface {
	io.Reader
	Peek(int) ([]byte, error)
}

var (
	ErrEmptyReader = errors.New("reader is empty")
)

// NonEmptyReader takes a reader and returns it if it is not empty, or
// `ErrEmptyReader` if it is empty. If there is an error when reading the first
// byte of the given reader, it will be propagated.
func NonEmptyReader(r io.Reader) (io.Reader, error) {
	pr, ok := r.(readPeeker)
	if !ok {
		pr = bufio.NewReader(r)
	}

	_, err := pr.Peek(1)
	if err == io.EOF {
		return nil, ErrEmptyReader
	}

	if err != nil {
		return nil, err
	}

	return pr, nil
}

type readCloser struct {
	io.Reader
	closer io.Closer
}

func (r *readCloser) Close() error {
	return r.closer.Close()
}

// NewReadCloser creates an `io.ReadCloser` with the given `io.Reader` and
// `io.Closer`.
func NewReadCloser(r io.Reader, c io.Closer) io.ReadCloser {
	return &readCloser{Reader: r, closer: c}
}

type writeNopCloser struct {
	io.Writer
}

func (writeNopCloser) Close() error { return nil }

// WriteNopCloser returns a WriteCloser with a no-op Close method wrapping
// the provided Writer w.
func WriteNopCloser(w io.Writer) io.WriteCloser {
	return writeNopCloser{w}
}

// CheckClose is used with defer to close the given io.Closer and check its
// returned error value. If Close returns an error and the given *error
// is not nil, *error is set to the error returned by Close.
//
// CheckClose is typically used with named return values like so:
//
//   func do(obj *Object) (err error) {
//     w, err := obj.Writer()
//     if err != nil {
//       return nil
//     }
//     defer CheckClose(w, &err)
//     // work with w
//   }
func CheckClose(c io.Closer, err *error) {
	if cerr := c.Close(); cerr != nil && *err == nil {
		*err = cerr
	}
}
