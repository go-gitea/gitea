package archiver

import (
	"fmt"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
)

// TarBrotli facilitates brotli compression of tarball archives.
type TarBrotli struct {
	*Tar
	Quality int
}

// CheckExt ensures the file extension matches the format.
func (*TarBrotli) CheckExt(filename string) error {
	if !strings.HasSuffix(filename, ".tar.br") &&
		!strings.HasSuffix(filename, ".tbr") {
		return fmt.Errorf("filename must have a .tar.br or .tbr extension")
	}
	return nil
}

// Archive creates a compressed tar file at destination
// containing the files listed in sources. The destination
// must end with ".tar.br" or ".tbr". File paths can be
// those of regular files or directories; directories will
// be recursively added.
func (tbr *TarBrotli) Archive(sources []string, destination string) error {
	err := tbr.CheckExt(destination)
	if err != nil {
		return fmt.Errorf("output %s", err.Error())
	}
	tbr.wrapWriter()
	return tbr.Tar.Archive(sources, destination)
}

// Unarchive unpacks the compressed tarball at
// source to destination. Destination will be
// treated as a folder name.
func (tbr *TarBrotli) Unarchive(source, destination string) error {
	tbr.wrapReader()
	return tbr.Tar.Unarchive(source, destination)
}

// Walk calls walkFn for each visited item in archive.
func (tbr *TarBrotli) Walk(archive string, walkFn WalkFunc) error {
	tbr.wrapReader()
	return tbr.Tar.Walk(archive, walkFn)
}

// Create opens txz for writing a compressed
// tar archive to out.
func (tbr *TarBrotli) Create(out io.Writer) error {
	tbr.wrapWriter()
	return tbr.Tar.Create(out)
}

// Open opens t for reading a compressed archive from
// in. The size parameter is not used.
func (tbr *TarBrotli) Open(in io.Reader, size int64) error {
	tbr.wrapReader()
	return tbr.Tar.Open(in, size)
}

// Extract extracts a single file from the tar archive.
// If the target is a directory, the entire folder will
// be extracted into destination.
func (tbr *TarBrotli) Extract(source, target, destination string) error {
	tbr.wrapReader()
	return tbr.Tar.Extract(source, target, destination)
}

func (tbr *TarBrotli) wrapWriter() {
	var brw *brotli.Writer
	tbr.Tar.writerWrapFn = func(w io.Writer) (io.Writer, error) {
		brw = brotli.NewWriterLevel(w, tbr.Quality)
		return brw, nil
	}
	tbr.Tar.cleanupWrapFn = func() {
		brw.Close()
	}
}

func (tbr *TarBrotli) wrapReader() {
	tbr.Tar.readerWrapFn = func(r io.Reader) (io.Reader, error) {
		return brotli.NewReader(r), nil
	}
}

func (tbr *TarBrotli) String() string { return "tar.br" }

// NewTarBrotli returns a new, default instance ready to be customized and used.
func NewTarBrotli() *TarBrotli {
	return &TarBrotli{
		Tar:     NewTar(),
		Quality: brotli.DefaultCompression,
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(TarBrotli))
	_ = Writer(new(TarBrotli))
	_ = Archiver(new(TarBrotli))
	_ = Unarchiver(new(TarBrotli))
	_ = Walker(new(TarBrotli))
	_ = Extractor(new(TarBrotli))
)

// DefaultTarBrotli is a convenient archiver ready to use.
var DefaultTarBrotli = NewTarBrotli()
