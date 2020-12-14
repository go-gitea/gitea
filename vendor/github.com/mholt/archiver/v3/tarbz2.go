package archiver

import (
	"fmt"
	"io"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

// TarBz2 facilitates bzip2 compression
// (https://github.com/dsnet/compress/blob/master/doc/bzip2-format.pdf)
// of tarball archives.
type TarBz2 struct {
	*Tar

	CompressionLevel int
}

// CheckExt ensures the file extension matches the format.
func (*TarBz2) CheckExt(filename string) error {
	if !strings.HasSuffix(filename, ".tar.bz2") &&
		!strings.HasSuffix(filename, ".tbz2") {
		return fmt.Errorf("filename must have a .tar.bz2 or .tbz2 extension")
	}
	return nil
}

// Archive creates a compressed tar file at destination
// containing the files listed in sources. The destination
// must end with ".tar.bz2" or ".tbz2". File paths can be
// those of regular files or directories; directories will
// be recursively added.
func (tbz2 *TarBz2) Archive(sources []string, destination string) error {
	err := tbz2.CheckExt(destination)
	if err != nil {
		return fmt.Errorf("output %s", err.Error())
	}
	tbz2.wrapWriter()
	return tbz2.Tar.Archive(sources, destination)
}

// Unarchive unpacks the compressed tarball at
// source to destination. Destination will be
// treated as a folder name.
func (tbz2 *TarBz2) Unarchive(source, destination string) error {
	tbz2.wrapReader()
	return tbz2.Tar.Unarchive(source, destination)
}

// Walk calls walkFn for each visited item in archive.
func (tbz2 *TarBz2) Walk(archive string, walkFn WalkFunc) error {
	tbz2.wrapReader()
	return tbz2.Tar.Walk(archive, walkFn)
}

// Create opens tbz2 for writing a compressed
// tar archive to out.
func (tbz2 *TarBz2) Create(out io.Writer) error {
	tbz2.wrapWriter()
	return tbz2.Tar.Create(out)
}

// Open opens t for reading a compressed archive from
// in. The size parameter is not used.
func (tbz2 *TarBz2) Open(in io.Reader, size int64) error {
	tbz2.wrapReader()
	return tbz2.Tar.Open(in, size)
}

// Extract extracts a single file from the tar archive.
// If the target is a directory, the entire folder will
// be extracted into destination.
func (tbz2 *TarBz2) Extract(source, target, destination string) error {
	tbz2.wrapReader()
	return tbz2.Tar.Extract(source, target, destination)
}

func (tbz2 *TarBz2) wrapWriter() {
	var bz2w *bzip2.Writer
	tbz2.Tar.writerWrapFn = func(w io.Writer) (io.Writer, error) {
		var err error
		bz2w, err = bzip2.NewWriter(w, &bzip2.WriterConfig{
			Level: tbz2.CompressionLevel,
		})
		return bz2w, err
	}
	tbz2.Tar.cleanupWrapFn = func() {
		bz2w.Close()
	}
}

func (tbz2 *TarBz2) wrapReader() {
	var bz2r *bzip2.Reader
	tbz2.Tar.readerWrapFn = func(r io.Reader) (io.Reader, error) {
		var err error
		bz2r, err = bzip2.NewReader(r, nil)
		return bz2r, err
	}
	tbz2.Tar.cleanupWrapFn = func() {
		bz2r.Close()
	}
}

func (tbz2 *TarBz2) String() string { return "tar.bz2" }

// NewTarBz2 returns a new, default instance ready to be customized and used.
func NewTarBz2() *TarBz2 {
	return &TarBz2{
		CompressionLevel: bzip2.DefaultCompression,
		Tar:              NewTar(),
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(TarBz2))
	_ = Writer(new(TarBz2))
	_ = Archiver(new(TarBz2))
	_ = Unarchiver(new(TarBz2))
	_ = Walker(new(TarBz2))
	_ = Extractor(new(TarBz2))
)

// DefaultTarBz2 is a convenient archiver ready to use.
var DefaultTarBz2 = NewTarBz2()
