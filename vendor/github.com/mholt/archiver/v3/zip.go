package archiver

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// ZipCompressionMethod Compression type
type ZipCompressionMethod uint16

// Compression methods.
// see https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT.
// Note LZMA: Disabled - because 7z isn't able to unpack ZIP+LZMA ZIP+LZMA2 archives made this way - and vice versa.
const (
	Store   ZipCompressionMethod = 0
	Deflate ZipCompressionMethod = 8
	BZIP2   ZipCompressionMethod = 12
	LZMA    ZipCompressionMethod = 14
	ZSTD    ZipCompressionMethod = 93
	XZ      ZipCompressionMethod = 95
)

// Zip provides facilities for operating ZIP archives.
// See https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT.
type Zip struct {
	// The compression level to use, as described
	// in the compress/flate package.
	CompressionLevel int

	// Whether to overwrite existing files; if false,
	// an error is returned if the file exists.
	OverwriteExisting bool

	// Whether to make all the directories necessary
	// to create a zip archive in the desired path.
	MkdirAll bool

	// If enabled, selective compression will only
	// compress files which are not already in a
	// compressed format; this is decided based
	// simply on file extension.
	SelectiveCompression bool

	// A single top-level folder can be implicitly
	// created by the Archive or Unarchive methods
	// if the files to be added to the archive
	// or the files to be extracted from the archive
	// do not all have a common root. This roughly
	// mimics the behavior of archival tools integrated
	// into OS file browsers which create a subfolder
	// to avoid unexpectedly littering the destination
	// folder with potentially many files, causing a
	// problematic cleanup/organization situation.
	// This feature is available for both creation
	// and extraction of archives, but may be slightly
	// inefficient with lots and lots of files,
	// especially on extraction.
	ImplicitTopLevelFolder bool

	// Strip number of leading paths. This feature is available
	// only during unpacking of the entire archive.
	StripComponents int

	// If true, errors encountered during reading
	// or writing a single file will be logged and
	// the operation will continue on remaining files.
	ContinueOnError bool

	// Compression algorithm
	FileMethod ZipCompressionMethod
	zw         *zip.Writer
	zr         *zip.Reader
	ridx       int
	//decinitialized bool
}

// CheckExt ensures the file extension matches the format.
func (*Zip) CheckExt(filename string) error {
	if !strings.HasSuffix(filename, ".zip") {
		return fmt.Errorf("filename must have a .zip extension")
	}
	return nil
}

// Registering a global decompressor is not reentrant and may panic
func registerDecompressor(zr *zip.Reader) {
	// register zstd decompressor
	zr.RegisterDecompressor(uint16(ZSTD), func(r io.Reader) io.ReadCloser {
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil
		}
		return zr.IOReadCloser()
	})
	zr.RegisterDecompressor(uint16(BZIP2), func(r io.Reader) io.ReadCloser {
		bz2r, err := bzip2.NewReader(r, nil)
		if err != nil {
			return nil
		}
		return bz2r
	})
	zr.RegisterDecompressor(uint16(XZ), func(r io.Reader) io.ReadCloser {
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil
		}
		return ioutil.NopCloser(xr)
	})
}

// CheckPath ensures the file extension matches the format.
func (*Zip) CheckPath(to, filename string) error {
	to, _ = filepath.Abs(to) //explicit the destination folder to prevent that 'string.HasPrefix' check can be 'bypassed' when no destination folder is supplied in input
	dest := filepath.Join(to, filename)
	//prevent path traversal attacks
	if !strings.HasPrefix(dest, to) {
		return &IllegalPathError{AbsolutePath: dest, Filename: filename}
	}
	return nil
}

// Archive creates a .zip file at destination containing
// the files listed in sources. The destination must end
// with ".zip". File paths can be those of regular files
// or directories. Regular files are stored at the 'root'
// of the archive, and directories are recursively added.
func (z *Zip) Archive(sources []string, destination string) error {
	err := z.CheckExt(destination)
	if err != nil {
		return fmt.Errorf("checking extension: %v", err)
	}
	if !z.OverwriteExisting && fileExists(destination) {
		return fmt.Errorf("file already exists: %s", destination)
	}

	// make the folder to contain the resulting archive
	// if it does not already exist
	destDir := filepath.Dir(destination)
	if z.MkdirAll && !fileExists(destDir) {
		err := mkdir(destDir, 0755)
		if err != nil {
			return fmt.Errorf("making folder for destination: %v", err)
		}
	}

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("creating %s: %v", destination, err)
	}
	defer out.Close()

	err = z.Create(out)
	if err != nil {
		return fmt.Errorf("creating zip: %v", err)
	}
	defer z.Close()

	var topLevelFolder string
	if z.ImplicitTopLevelFolder && multipleTopLevels(sources) {
		topLevelFolder = folderNameFromFileName(destination)
	}

	for _, source := range sources {
		err := z.writeWalk(source, topLevelFolder, destination)
		if err != nil {
			return fmt.Errorf("walking %s: %v", source, err)
		}
	}

	return nil
}

// Unarchive unpacks the .zip file at source to destination.
// Destination will be treated as a folder name.
func (z *Zip) Unarchive(source, destination string) error {
	if !fileExists(destination) && z.MkdirAll {
		err := mkdir(destination, 0755)
		if err != nil {
			return fmt.Errorf("preparing destination: %v", err)
		}
	}

	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("opening source file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("statting source file: %v", err)
	}

	err = z.Open(file, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("opening zip archive for reading: %v", err)
	}
	defer z.Close()

	// if the files in the archive do not all share a common
	// root, then make sure we extract to a single subfolder
	// rather than potentially littering the destination...
	if z.ImplicitTopLevelFolder {
		files := make([]string, len(z.zr.File))
		for i := range z.zr.File {
			files[i] = z.zr.File[i].Name
		}
		if multipleTopLevels(files) {
			destination = filepath.Join(destination, folderNameFromFileName(source))
		}
	}

	for {
		err := z.extractNext(destination)
		if err == io.EOF {
			break
		}
		if err != nil {
			if z.ContinueOnError || IsIllegalPathError(err) {
				log.Printf("[ERROR] Reading file in zip archive: %v", err)
				continue
			}
			return fmt.Errorf("reading file in zip archive: %v", err)
		}
	}

	return nil
}

func (z *Zip) extractNext(to string) error {
	f, err := z.Read()
	if err != nil {
		return err // don't wrap error; calling loop must break on io.EOF
	}
	defer f.Close()

	header, ok := f.Header.(zip.FileHeader)
	if !ok {
		return fmt.Errorf("expected header to be zip.FileHeader but was %T", f.Header)
	}

	errPath := z.CheckPath(to, header.Name)
	if errPath != nil {
		return fmt.Errorf("checking path traversal attempt: %v", errPath)
	}

	if z.StripComponents > 0 {
		if strings.Count(header.Name, "/") < z.StripComponents {
			return nil // skip path with fewer components
		}

		for i := 0; i < z.StripComponents; i++ {
			slash := strings.Index(header.Name, "/")
			header.Name = header.Name[slash+1:]
		}
	}
	return z.extractFile(f, to, &header)
}

func (z *Zip) extractFile(f File, to string, header *zip.FileHeader) error {
	to = filepath.Join(to, header.Name)

	// if a directory, no content; simply make the directory and return
	if f.IsDir() {
		return mkdir(to, f.Mode())
	}

	// do not overwrite existing files, if configured
	if !z.OverwriteExisting && fileExists(to) {
		return fmt.Errorf("file already exists: %s", to)
	}

	// extract symbolic links as symbolic links
	if isSymlink(header.FileInfo()) {
		// symlink target is the contents of the file
		buf := new(bytes.Buffer)
		_, err := io.Copy(buf, f)
		if err != nil {
			return fmt.Errorf("%s: reading symlink target: %v", header.Name, err)
		}
		return writeNewSymbolicLink(to, strings.TrimSpace(buf.String()))
	}

	return writeNewFile(to, f, f.Mode())
}

func (z *Zip) writeWalk(source, topLevelFolder, destination string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("%s: stat: %v", source, err)
	}
	destAbs, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("%s: getting absolute path of destination %s: %v", source, destination, err)
	}

	return filepath.Walk(source, func(fpath string, info os.FileInfo, err error) error {
		handleErr := func(err error) error {
			if z.ContinueOnError {
				log.Printf("[ERROR] Walking %s: %v", fpath, err)
				return nil
			}
			return err
		}
		if err != nil {
			return handleErr(fmt.Errorf("traversing %s: %v", fpath, err))
		}
		if info == nil {
			return handleErr(fmt.Errorf("%s: no file info", fpath))
		}

		// make sure we do not copy the output file into the output
		// file; that results in an infinite loop and disk exhaustion!
		fpathAbs, err := filepath.Abs(fpath)
		if err != nil {
			return handleErr(fmt.Errorf("%s: getting absolute path: %v", fpath, err))
		}
		if within(fpathAbs, destAbs) {
			return nil
		}

		// build the name to be used within the archive
		nameInArchive, err := makeNameInArchive(sourceInfo, source, topLevelFolder, fpath)
		if err != nil {
			return handleErr(err)
		}

		var file io.ReadCloser
		if info.Mode().IsRegular() {
			file, err = os.Open(fpath)
			if err != nil {
				return handleErr(fmt.Errorf("%s: opening: %v", fpath, err))
			}
			defer file.Close()
		}
		err = z.Write(File{
			FileInfo: FileInfo{
				FileInfo:   info,
				CustomName: nameInArchive,
			},
			ReadCloser: file,
		})
		if err != nil {
			return handleErr(fmt.Errorf("%s: writing: %s", fpath, err))
		}

		return nil
	})
}

// Create opens z for writing a ZIP archive to out.
func (z *Zip) Create(out io.Writer) error {
	if z.zw != nil {
		return fmt.Errorf("zip archive is already created for writing")
	}
	z.zw = zip.NewWriter(out)
	if z.CompressionLevel != flate.DefaultCompression {
		z.zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(out, z.CompressionLevel)
		})
	}
	switch z.FileMethod {
	case BZIP2:
		z.zw.RegisterCompressor(uint16(BZIP2), func(out io.Writer) (io.WriteCloser, error) {
			return bzip2.NewWriter(out, &bzip2.WriterConfig{Level: z.CompressionLevel})
		})
	case ZSTD:
		z.zw.RegisterCompressor(uint16(ZSTD), func(out io.Writer) (io.WriteCloser, error) {
			return zstd.NewWriter(out)
		})
	case XZ:
		z.zw.RegisterCompressor(uint16(XZ), func(out io.Writer) (io.WriteCloser, error) {
			return xz.NewWriter(out)
		})
	}
	return nil
}

// Write writes f to z, which must have been opened for writing first.
func (z *Zip) Write(f File) error {
	if z.zw == nil {
		return fmt.Errorf("zip archive was not created for writing first")
	}
	if f.FileInfo == nil {
		return fmt.Errorf("no file info")
	}
	if f.FileInfo.Name() == "" {
		return fmt.Errorf("missing file name")
	}

	header, err := zip.FileInfoHeader(f)
	if err != nil {
		return fmt.Errorf("%s: getting header: %v", f.Name(), err)
	}

	if f.IsDir() {
		header.Name += "/" // required - strangely no mention of this in zip spec? but is in godoc...
		header.Method = zip.Store
	} else {
		ext := strings.ToLower(path.Ext(header.Name))
		if _, ok := compressedFormats[ext]; ok && z.SelectiveCompression {
			header.Method = zip.Store
		} else {
			header.Method = uint16(z.FileMethod)
		}
	}

	writer, err := z.zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("%s: making header: %v", f.Name(), err)
	}

	return z.writeFile(f, writer)
}

func (z *Zip) writeFile(f File, writer io.Writer) error {
	if f.IsDir() {
		return nil // directories have no contents
	}
	if isSymlink(f) {
		// file body for symlinks is the symlink target
		linkTarget, err := os.Readlink(f.Name())
		if err != nil {
			return fmt.Errorf("%s: readlink: %v", f.Name(), err)
		}
		_, err = writer.Write([]byte(filepath.ToSlash(linkTarget)))
		if err != nil {
			return fmt.Errorf("%s: writing symlink target: %v", f.Name(), err)
		}
		return nil
	}

	if f.ReadCloser == nil {
		return fmt.Errorf("%s: no way to read file contents", f.Name())
	}
	_, err := io.Copy(writer, f)
	if err != nil {
		return fmt.Errorf("%s: copying contents: %v", f.Name(), err)
	}

	return nil
}

// Open opens z for reading an archive from in,
// which is expected to have the given size and
// which must be an io.ReaderAt.
func (z *Zip) Open(in io.Reader, size int64) error {
	inRdrAt, ok := in.(io.ReaderAt)
	if !ok {
		return fmt.Errorf("reader must be io.ReaderAt")
	}
	if z.zr != nil {
		return fmt.Errorf("zip archive is already open for reading")
	}
	var err error
	z.zr, err = zip.NewReader(inRdrAt, size)
	if err != nil {
		return fmt.Errorf("creating reader: %v", err)
	}
	registerDecompressor(z.zr)
	z.ridx = 0
	return nil
}

// Read reads the next file from z, which must have
// already been opened for reading. If there are no
// more files, the error is io.EOF. The File must
// be closed when finished reading from it.
func (z *Zip) Read() (File, error) {
	if z.zr == nil {
		return File{}, fmt.Errorf("zip archive is not open")
	}
	if z.ridx >= len(z.zr.File) {
		return File{}, io.EOF
	}

	// access the file and increment counter so that
	// if there is an error processing this file, the
	// caller can still iterate to the next file
	zf := z.zr.File[z.ridx]
	z.ridx++

	file := File{
		FileInfo: zf.FileInfo(),
		Header:   zf.FileHeader,
	}

	rc, err := zf.Open()
	if err != nil {
		return file, fmt.Errorf("%s: open compressed file: %v", zf.Name, err)
	}
	file.ReadCloser = rc

	return file, nil
}

// Close closes the zip archive(s) opened by Create and Open.
func (z *Zip) Close() error {
	if z.zr != nil {
		z.zr = nil
	}
	if z.zw != nil {
		zw := z.zw
		z.zw = nil
		return zw.Close()
	}
	return nil
}

// Walk calls walkFn for each visited item in archive.
func (z *Zip) Walk(archive string, walkFn WalkFunc) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("opening zip reader: %v", err)
	}
	defer zr.Close()
	registerDecompressor(&zr.Reader)
	for _, zf := range zr.File {
		zfrc, err := zf.Open()
		if err != nil {
			if zfrc != nil {
				zfrc.Close()
			}
			if z.ContinueOnError {
				log.Printf("[ERROR] Opening %s: %v", zf.Name, err)
				continue
			}
			return fmt.Errorf("opening %s: %v", zf.Name, err)
		}

		err = walkFn(File{
			FileInfo:   zf.FileInfo(),
			Header:     zf.FileHeader,
			ReadCloser: zfrc,
		})
		zfrc.Close()
		if err != nil {
			if err == ErrStopWalk {
				break
			}
			if z.ContinueOnError {
				log.Printf("[ERROR] Walking %s: %v", zf.Name, err)
				continue
			}
			return fmt.Errorf("walking %s: %v", zf.Name, err)
		}
	}

	return nil
}

// Extract extracts a single file from the zip archive.
// If the target is a directory, the entire folder will
// be extracted into destination.
func (z *Zip) Extract(source, target, destination string) error {
	// target refers to a path inside the archive, which should be clean also
	target = path.Clean(target)

	// if the target ends up being a directory, then
	// we will continue walking and extracting files
	// until we are no longer within that directory
	var targetDirPath string

	return z.Walk(source, func(f File) error {
		zfh, ok := f.Header.(zip.FileHeader)
		if !ok {
			return fmt.Errorf("expected header to be zip.FileHeader but was %T", f.Header)
		}

		// importantly, cleaning the path strips tailing slash,
		// which must be appended to folders within the archive
		name := path.Clean(zfh.Name)
		if f.IsDir() && target == name {
			targetDirPath = path.Dir(name)
		}

		if within(target, zfh.Name) {
			// either this is the exact file we want, or is
			// in the directory we want to extract

			// build the filename we will extract to
			end, err := filepath.Rel(targetDirPath, zfh.Name)
			if err != nil {
				return fmt.Errorf("relativizing paths: %v", err)
			}
			joined := filepath.Join(destination, end)

			err = z.extractFile(f, joined, &zfh)
			if err != nil {
				return fmt.Errorf("extracting file %s: %v", zfh.Name, err)
			}

			// if our target was not a directory, stop walk
			if targetDirPath == "" {
				return ErrStopWalk
			}
		} else if targetDirPath != "" {
			// finished walking the entire directory
			return ErrStopWalk
		}

		return nil
	})
}

// Match returns true if the format of file matches this
// type's format. It should not affect reader position.
func (*Zip) Match(file io.ReadSeeker) (bool, error) {
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, err
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return false, err
	}
	defer func() {
		_, _ = file.Seek(currentPos, io.SeekStart)
	}()

	buf := make([]byte, 4)
	if n, err := file.Read(buf); err != nil || n < 4 {
		return false, nil
	}
	return bytes.Equal(buf, []byte("PK\x03\x04")), nil
}

func (z *Zip) String() string { return "zip" }

// NewZip returns a new, default instance ready to be customized and used.
func NewZip() *Zip {
	return &Zip{
		CompressionLevel:     flate.DefaultCompression,
		MkdirAll:             true,
		SelectiveCompression: true,
		FileMethod:           Deflate,
	}
}

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(Zip))
	_ = Writer(new(Zip))
	_ = Archiver(new(Zip))
	_ = Unarchiver(new(Zip))
	_ = Walker(new(Zip))
	_ = Extractor(new(Zip))
	_ = Matcher(new(Zip))
	_ = ExtensionChecker(new(Zip))
	_ = FilenameChecker(new(Zip))
)

// compressedFormats is a (non-exhaustive) set of lowercased
// file extensions for formats that are typically already
// compressed. Compressing files that are already compressed
// is inefficient, so use this set of extension to avoid that.
var compressedFormats = map[string]struct{}{
	".7z":   {},
	".avi":  {},
	".br":   {},
	".bz2":  {},
	".cab":  {},
	".docx": {},
	".gif":  {},
	".gz":   {},
	".jar":  {},
	".jpeg": {},
	".jpg":  {},
	".lz":   {},
	".lz4":  {},
	".lzma": {},
	".m4v":  {},
	".mov":  {},
	".mp3":  {},
	".mp4":  {},
	".mpeg": {},
	".mpg":  {},
	".png":  {},
	".pptx": {},
	".rar":  {},
	".sz":   {},
	".tbz2": {},
	".tgz":  {},
	".tsz":  {},
	".txz":  {},
	".xlsx": {},
	".xz":   {},
	".zip":  {},
	".zipx": {},
}

// DefaultZip is a default instance that is conveniently ready to use.
var DefaultZip = NewZip()
