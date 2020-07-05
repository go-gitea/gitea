package archiver

import (
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	pgzip "github.com/klauspost/pgzip"
)

// TarGz facilitates gzip compression
// (RFC 1952) of tarball archives.
type TarGz struct {
	*Tar

	// The compression level to use, as described
	// in the compress/gzip package.
	CompressionLevel int

	// Disables parallel gzip.
	SingleThreaded bool
}

// CheckExt ensures the file extension matches the format.
func (*TarGz) CheckExt(filename string) error {
	if !strings.HasSuffix(filename, ".tar.gz") &&
		!strings.HasSuffix(filename, ".tgz") {
		return fmt.Errorf("filename must have a .tar.gz or .tgz extension")
	}
	return nil
}

// Archive creates a compressed tar file at destination
// containing the files listed in sources. The destination
// must end with ".tar.gz" or ".tgz". File paths can be
// those of regular files or directories; directories will
// be recursively added.
func (tgz *TarGz) Archive(sources []string, destination string) error {
	err := tgz.CheckExt(destination)
	if err != nil {
		return fmt.Errorf("output %s", err.Error())
	}
	tgz.wrapWriter()
	return tgz.Tar.Archive(sources, destination)
}

// Unarchive unpacks the compressed tarball at
// source to destination. Destination will be
// treated as a folder name.
func (tgz *TarGz) Unarchive(source, destination string) error {
	tgz.wrapReader()
	return tgz.Tar.Unarchive(source, destination)
}

// Walk calls walkFn for each visited item in archive.
func (tgz *TarGz) Walk(archive string, walkFn WalkFunc) error {
	tgz.wrapReader()
	return tgz.Tar.Walk(archive, walkFn)
}

// Create opens txz for writing a compressed
// tar archive to out.
func (tgz *TarGz) Create(out io.Writer) error {
	tgz.wrapWriter()
	return tgz.Tar.Create(out)
}

// Open opens t for reading a compressed archive from
// in. The size parameter is not used.
func (tgz *TarGz) Open(in io.Reader, size int64) error {
	tgz.wrapReader()
	return tgz.Tar.Open(in, size)
}

// Extract extracts a single file from the tar archive.
// If the target is a directory, the entire folder will
// be extracted into destination.
func (tgz *TarGz) Extract(source, target, destination string) error {
	tgz.wrapReader()
	return tgz.Tar.Extract(source, target, destination)
}

func (tgz *TarGz) wrapWriter() {
	var gzw io.WriteCloser
	tgz.Tar.writerWrapFn = func(w io.Writer) (io.Writer, error) {
		var err error
		if tgz.SingleThreaded {
			gzw, err = gzip.NewWriterLevel(w, tgz.CompressionLevel)
		} else {
			gzw, err = pgzip.NewWriterLevel(w, tgz.CompressionLevel)
		}
		return gzw, err
	}
	tgz.Tar.cleanupWrapFn = func() {
		gzw.Close()
	}
}

func (tgz *TarGz) wrapReader() {
	var gzr io.ReadCloser
	tgz.Tar.readerWrapFn = func(r io.Reader) (io.Reader, error) {
		var err error
		if tgz.SingleThreaded {
			gzr, err = gzip.NewReader(r)
		} else {
			gzr, err = pgzip.NewReader(r)
		}
		return gzr, err
	}
	tgz.Tar.cleanupWrapFn = func() {
		gzr.Close()
	}
}

func (tgz *TarGz) String() string { return "tar.gz" }

// NewTarGz returns a new, default instance ready to be customized and used.
func NewTarGz() *TarGz {
	return &TarGz{
		CompressionLevel: gzip.DefaultCompression,
		Tar:              NewTar(),
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(TarGz))
	_ = Writer(new(TarGz))
	_ = Archiver(new(TarGz))
	_ = Unarchiver(new(TarGz))
	_ = Walker(new(TarGz))
	_ = Extractor(new(TarGz))
)

// DefaultTarGz is a convenient archiver ready to use.
var DefaultTarGz = NewTarGz()
