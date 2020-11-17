package archiver

import (
	"fmt"
	"io"
	"strings"

	"github.com/ulikunitz/xz"
	fastxz "github.com/xi2/xz"
)

// TarXz facilitates xz compression
// (https://tukaani.org/xz/format.html)
// of tarball archives.
type TarXz struct {
	*Tar
}

// CheckExt ensures the file extension matches the format.
func (*TarXz) CheckExt(filename string) error {
	if !strings.HasSuffix(filename, ".tar.xz") &&
		!strings.HasSuffix(filename, ".txz") {
		return fmt.Errorf("filename must have a .tar.xz or .txz extension")
	}
	return nil
}

// Archive creates a compressed tar file at destination
// containing the files listed in sources. The destination
// must end with ".tar.xz" or ".txz". File paths can be
// those of regular files or directories; directories will
// be recursively added.
func (txz *TarXz) Archive(sources []string, destination string) error {
	err := txz.CheckExt(destination)
	if err != nil {
		return fmt.Errorf("output %s", err.Error())
	}
	txz.wrapWriter()
	return txz.Tar.Archive(sources, destination)
}

// Unarchive unpacks the compressed tarball at
// source to destination. Destination will be
// treated as a folder name.
func (txz *TarXz) Unarchive(source, destination string) error {
	txz.wrapReader()
	return txz.Tar.Unarchive(source, destination)
}

// Walk calls walkFn for each visited item in archive.
func (txz *TarXz) Walk(archive string, walkFn WalkFunc) error {
	txz.wrapReader()
	return txz.Tar.Walk(archive, walkFn)
}

// Create opens txz for writing a compressed
// tar archive to out.
func (txz *TarXz) Create(out io.Writer) error {
	txz.wrapWriter()
	return txz.Tar.Create(out)
}

// Open opens t for reading a compressed archive from
// in. The size parameter is not used.
func (txz *TarXz) Open(in io.Reader, size int64) error {
	txz.wrapReader()
	return txz.Tar.Open(in, size)
}

// Extract extracts a single file from the tar archive.
// If the target is a directory, the entire folder will
// be extracted into destination.
func (txz *TarXz) Extract(source, target, destination string) error {
	txz.wrapReader()
	return txz.Tar.Extract(source, target, destination)
}

func (txz *TarXz) wrapWriter() {
	var xzw *xz.Writer
	txz.Tar.writerWrapFn = func(w io.Writer) (io.Writer, error) {
		var err error
		xzw, err = xz.NewWriter(w)
		return xzw, err
	}
	txz.Tar.cleanupWrapFn = func() {
		xzw.Close()
	}
}

func (txz *TarXz) wrapReader() {
	var xzr *fastxz.Reader
	txz.Tar.readerWrapFn = func(r io.Reader) (io.Reader, error) {
		var err error
		xzr, err = fastxz.NewReader(r, 0)
		return xzr, err
	}
}

func (txz *TarXz) String() string { return "tar.xz" }

// NewTarXz returns a new, default instance ready to be customized and used.
func NewTarXz() *TarXz {
	return &TarXz{
		Tar: NewTar(),
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(TarXz))
	_ = Writer(new(TarXz))
	_ = Archiver(new(TarXz))
	_ = Unarchiver(new(TarXz))
	_ = Walker(new(TarXz))
	_ = Extractor(new(TarXz))
)

// DefaultTarXz is a convenient archiver ready to use.
var DefaultTarXz = NewTarXz()
