package archiver

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/andybalholm/brotli"
)

// Brotli facilitates brotli compression.
type Brotli struct {
	Quality int
}

// Compress reads in, compresses it, and writes it to out.
func (br *Brotli) Compress(in io.Reader, out io.Writer) error {
	w := brotli.NewWriterLevel(out, br.Quality)
	defer w.Close()
	_, err := io.Copy(w, in)
	return err
}

// Decompress reads in, decompresses it, and writes it to out.
func (br *Brotli) Decompress(in io.Reader, out io.Writer) error {
	r := brotli.NewReader(in)
	_, err := io.Copy(out, r)
	return err
}

// CheckExt ensures the file extension matches the format.
func (br *Brotli) CheckExt(filename string) error {
	if filepath.Ext(filename) != ".br" {
		return fmt.Errorf("filename must have a .br extension")
	}
	return nil
}

func (br *Brotli) String() string { return "brotli" }

// NewBrotli returns a new, default instance ready to be customized and used.
func NewBrotli() *Brotli {
	return &Brotli{
		Quality: brotli.DefaultCompression,
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Compressor(new(Brotli))
	_ = Decompressor(new(Brotli))
)

// DefaultBrotli is a default instance that is conveniently ready to use.
var DefaultBrotli = NewBrotli()
