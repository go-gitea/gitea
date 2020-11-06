package archiver

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

// Zstd facilitates Zstandard compression.
type Zstd struct {
	EncoderOptions []zstd.EOption
	DecoderOptions []zstd.DOption
}

// Compress reads in, compresses it, and writes it to out.
func (zs *Zstd) Compress(in io.Reader, out io.Writer) error {
	w, err := zstd.NewWriter(out, zs.EncoderOptions...)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, in)
	return err
}

// Decompress reads in, decompresses it, and writes it to out.
func (zs *Zstd) Decompress(in io.Reader, out io.Writer) error {
	r, err := zstd.NewReader(in, zs.DecoderOptions...)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(out, r)
	return err
}

// CheckExt ensures the file extension matches the format.
func (zs *Zstd) CheckExt(filename string) error {
	if filepath.Ext(filename) != ".zst" {
		return fmt.Errorf("filename must have a .zst extension")
	}
	return nil
}

func (zs *Zstd) String() string { return "zstd" }

// NewZstd returns a new, default instance ready to be customized and used.
func NewZstd() *Zstd {
	return new(Zstd)
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Compressor(new(Zstd))
	_ = Decompressor(new(Zstd))
)

// DefaultZstd is a default instance that is conveniently ready to use.
var DefaultZstd = NewZstd()
