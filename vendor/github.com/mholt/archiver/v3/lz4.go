package archiver

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/pierrec/lz4"
)

// Lz4 facilitates LZ4 compression.
type Lz4 struct {
	CompressionLevel int
}

// Compress reads in, compresses it, and writes it to out.
func (lz *Lz4) Compress(in io.Reader, out io.Writer) error {
	w := lz4.NewWriter(out)
	w.Header.CompressionLevel = lz.CompressionLevel
	defer w.Close()
	_, err := io.Copy(w, in)
	return err
}

// Decompress reads in, decompresses it, and writes it to out.
func (lz *Lz4) Decompress(in io.Reader, out io.Writer) error {
	r := lz4.NewReader(in)
	_, err := io.Copy(out, r)
	return err
}

// CheckExt ensures the file extension matches the format.
func (lz *Lz4) CheckExt(filename string) error {
	if filepath.Ext(filename) != ".lz4" {
		return fmt.Errorf("filename must have a .lz4 extension")
	}
	return nil
}

func (lz *Lz4) String() string { return "lz4" }

// NewLz4 returns a new, default instance ready to be customized and used.
func NewLz4() *Lz4 {
	return &Lz4{
		CompressionLevel: 9, // https://github.com/lz4/lz4/blob/1b819bfd633ae285df2dfe1b0589e1ec064f2873/lib/lz4hc.h#L48
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Compressor(new(Lz4))
	_ = Decompressor(new(Lz4))
)

// DefaultLz4 is a default instance that is conveniently ready to use.
var DefaultLz4 = NewLz4()
