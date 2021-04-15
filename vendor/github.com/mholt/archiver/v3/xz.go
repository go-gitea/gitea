package archiver

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/ulikunitz/xz"
	fastxz "github.com/xi2/xz"
)

// Xz facilitates XZ compression.
type Xz struct{}

// Compress reads in, compresses it, and writes it to out.
func (x *Xz) Compress(in io.Reader, out io.Writer) error {
	w, err := xz.NewWriter(out)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, in)
	return err
}

// Decompress reads in, decompresses it, and writes it to out.
func (x *Xz) Decompress(in io.Reader, out io.Writer) error {
	r, err := fastxz.NewReader(in, 0)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	return err
}

// CheckExt ensures the file extension matches the format.
func (x *Xz) CheckExt(filename string) error {
	if filepath.Ext(filename) != ".xz" {
		return fmt.Errorf("filename must have a .xz extension")
	}
	return nil
}

func (x *Xz) String() string { return "xz" }

// NewXz returns a new, default instance ready to be customized and used.
func NewXz() *Xz {
	return new(Xz)
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Compressor(new(Xz))
	_ = Decompressor(new(Xz))
)

// DefaultXz is a default instance that is conveniently ready to use.
var DefaultXz = NewXz()
