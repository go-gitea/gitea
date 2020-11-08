package archiver

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/snappy"
)

// TarSz facilitates Snappy compression
// (https://github.com/google/snappy)
// of tarball archives.
type TarSz struct {
	*Tar
}

// CheckExt ensures the file extension matches the format.
func (*TarSz) CheckExt(filename string) error {
	if !strings.HasSuffix(filename, ".tar.sz") &&
		!strings.HasSuffix(filename, ".tsz") {
		return fmt.Errorf("filename must have a .tar.sz or .tsz extension")
	}
	return nil
}

// Archive creates a compressed tar file at destination
// containing the files listed in sources. The destination
// must end with ".tar.sz" or ".tsz". File paths can be
// those of regular files or directories; directories will
// be recursively added.
func (tsz *TarSz) Archive(sources []string, destination string) error {
	err := tsz.CheckExt(destination)
	if err != nil {
		return fmt.Errorf("output %s", err.Error())
	}
	tsz.wrapWriter()
	return tsz.Tar.Archive(sources, destination)
}

// Unarchive unpacks the compressed tarball at
// source to destination. Destination will be
// treated as a folder name.
func (tsz *TarSz) Unarchive(source, destination string) error {
	tsz.wrapReader()
	return tsz.Tar.Unarchive(source, destination)
}

// Walk calls walkFn for each visited item in archive.
func (tsz *TarSz) Walk(archive string, walkFn WalkFunc) error {
	tsz.wrapReader()
	return tsz.Tar.Walk(archive, walkFn)
}

// Create opens tsz for writing a compressed
// tar archive to out.
func (tsz *TarSz) Create(out io.Writer) error {
	tsz.wrapWriter()
	return tsz.Tar.Create(out)
}

// Open opens t for reading a compressed archive from
// in. The size parameter is not used.
func (tsz *TarSz) Open(in io.Reader, size int64) error {
	tsz.wrapReader()
	return tsz.Tar.Open(in, size)
}

// Extract extracts a single file from the tar archive.
// If the target is a directory, the entire folder will
// be extracted into destination.
func (tsz *TarSz) Extract(source, target, destination string) error {
	tsz.wrapReader()
	return tsz.Tar.Extract(source, target, destination)
}

func (tsz *TarSz) wrapWriter() {
	var sw *snappy.Writer
	tsz.Tar.writerWrapFn = func(w io.Writer) (io.Writer, error) {
		sw = snappy.NewBufferedWriter(w)
		return sw, nil
	}
	tsz.Tar.cleanupWrapFn = func() {
		sw.Close()
	}
}

func (tsz *TarSz) wrapReader() {
	tsz.Tar.readerWrapFn = func(r io.Reader) (io.Reader, error) {
		return snappy.NewReader(r), nil
	}
}

func (tsz *TarSz) String() string { return "tar.sz" }

// NewTarSz returns a new, default instance ready to be customized and used.
func NewTarSz() *TarSz {
	return &TarSz{
		Tar: NewTar(),
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(TarSz))
	_ = Writer(new(TarSz))
	_ = Archiver(new(TarSz))
	_ = Unarchiver(new(TarSz))
	_ = Walker(new(TarSz))
	_ = Extractor(new(TarSz))
)

// DefaultTarSz is a convenient archiver ready to use.
var DefaultTarSz = NewTarSz()
