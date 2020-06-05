package archiver

import (
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// TarZstd facilitates Zstandard compression
// (RFC 8478) of tarball archives.
type TarZstd struct {
	*Tar
}

// CheckExt ensures the file extension matches the format.
func (*TarZstd) CheckExt(filename string) error {
	if !strings.HasSuffix(filename, ".tar.zst") {
		return fmt.Errorf("filename must have a .tar.zst extension")
	}
	return nil
}

// Archive creates a compressed tar file at destination
// containing the files listed in sources. The destination
// must end with ".tar.zst" or ".tzst". File paths can be
// those of regular files or directories; directories will
// be recursively added.
func (tzst *TarZstd) Archive(sources []string, destination string) error {
	err := tzst.CheckExt(destination)
	if err != nil {
		return fmt.Errorf("output %s", err.Error())
	}
	tzst.wrapWriter()
	return tzst.Tar.Archive(sources, destination)
}

// Unarchive unpacks the compressed tarball at
// source to destination. Destination will be
// treated as a folder name.
func (tzst *TarZstd) Unarchive(source, destination string) error {
	tzst.wrapReader()
	return tzst.Tar.Unarchive(source, destination)
}

// Walk calls walkFn for each visited item in archive.
func (tzst *TarZstd) Walk(archive string, walkFn WalkFunc) error {
	tzst.wrapReader()
	return tzst.Tar.Walk(archive, walkFn)
}

// Create opens txz for writing a compressed
// tar archive to out.
func (tzst *TarZstd) Create(out io.Writer) error {
	tzst.wrapWriter()
	return tzst.Tar.Create(out)
}

// Open opens t for reading a compressed archive from
// in. The size parameter is not used.
func (tzst *TarZstd) Open(in io.Reader, size int64) error {
	tzst.wrapReader()
	return tzst.Tar.Open(in, size)
}

// Extract extracts a single file from the tar archive.
// If the target is a directory, the entire folder will
// be extracted into destination.
func (tzst *TarZstd) Extract(source, target, destination string) error {
	tzst.wrapReader()
	return tzst.Tar.Extract(source, target, destination)
}

func (tzst *TarZstd) wrapWriter() {
	var zstdw *zstd.Encoder
	tzst.Tar.writerWrapFn = func(w io.Writer) (io.Writer, error) {
		var err error
		zstdw, err = zstd.NewWriter(w)
		return zstdw, err
	}
	tzst.Tar.cleanupWrapFn = func() {
		zstdw.Close()
	}
}

func (tzst *TarZstd) wrapReader() {
	var zstdr *zstd.Decoder
	tzst.Tar.readerWrapFn = func(r io.Reader) (io.Reader, error) {
		var err error
		zstdr, err = zstd.NewReader(r)
		return zstdr, err
	}
	tzst.Tar.cleanupWrapFn = func() {
		zstdr.Close()
	}
}

func (tzst *TarZstd) String() string { return "tar.zst" }

// NewTarZstd returns a new, default instance ready to be customized and used.
func NewTarZstd() *TarZstd {
	return &TarZstd{
		Tar: NewTar(),
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(TarZstd))
	_ = Writer(new(TarZstd))
	_ = Archiver(new(TarZstd))
	_ = Unarchiver(new(TarZstd))
	_ = Walker(new(TarZstd))
	_ = ExtensionChecker(new(TarZstd))
	_ = Extractor(new(TarZstd))
)

// DefaultTarZstd is a convenient archiver ready to use.
var DefaultTarZstd = NewTarZstd()
