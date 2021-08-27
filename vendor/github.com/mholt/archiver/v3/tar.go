package archiver

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Tar provides facilities for operating TAR archives.
// See http://www.gnu.org/software/tar/manual/html_node/Standard.html.
type Tar struct {
	// Whether to overwrite existing files; if false,
	// an error is returned if the file exists.
	OverwriteExisting bool

	// Whether to make all the directories necessary
	// to create a tar archive in the desired path.
	MkdirAll bool

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

	tw *tar.Writer
	tr *tar.Reader

	readerWrapFn  func(io.Reader) (io.Reader, error)
	writerWrapFn  func(io.Writer) (io.Writer, error)
	cleanupWrapFn func()
}

// CheckExt ensures the file extension matches the format.
func (*Tar) CheckExt(filename string) error {
	if !strings.HasSuffix(filename, ".tar") {
		return fmt.Errorf("filename must have a .tar extension")
	}
	return nil
}

// CheckPath ensures that the filename has not been crafted to perform path traversal attacks
func (*Tar) CheckPath(to, filename string) error {
	to, _ = filepath.Abs(to) //explicit the destination folder to prevent that 'string.HasPrefix' check can be 'bypassed' when no destination folder is supplied in input
	dest := filepath.Join(to, filename)
	//prevent path traversal attacks
	if !strings.HasPrefix(dest, to) {
		return &IllegalPathError{AbsolutePath: dest, Filename: filename}
	}
	return nil
}

// Archive creates a tarball file at destination containing
// the files listed in sources. The destination must end with
// ".tar". File paths can be those of regular files or
// directories; directories will be recursively added.
func (t *Tar) Archive(sources []string, destination string) error {
	err := t.CheckExt(destination)
	if t.writerWrapFn == nil && err != nil {
		return fmt.Errorf("checking extension: %v", err)
	}
	if !t.OverwriteExisting && fileExists(destination) {
		return fmt.Errorf("file already exists: %s", destination)
	}

	// make the folder to contain the resulting archive
	// if it does not already exist
	destDir := filepath.Dir(destination)
	if t.MkdirAll && !fileExists(destDir) {
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

	err = t.Create(out)
	if err != nil {
		return fmt.Errorf("creating tar: %v", err)
	}
	defer t.Close()

	var topLevelFolder string
	if t.ImplicitTopLevelFolder && multipleTopLevels(sources) {
		topLevelFolder = folderNameFromFileName(destination)
	}

	for _, source := range sources {
		err := t.writeWalk(source, topLevelFolder, destination)
		if err != nil {
			return fmt.Errorf("walking %s: %v", source, err)
		}
	}

	return nil
}

// Unarchive unpacks the .tar file at source to destination.
// Destination will be treated as a folder name.
func (t *Tar) Unarchive(source, destination string) error {
	if !fileExists(destination) && t.MkdirAll {
		err := mkdir(destination, 0755)
		if err != nil {
			return fmt.Errorf("preparing destination: %v", err)
		}
	}

	// if the files in the archive do not all share a common
	// root, then make sure we extract to a single subfolder
	// rather than potentially littering the destination...
	if t.ImplicitTopLevelFolder {
		var err error
		destination, err = t.addTopLevelFolder(source, destination)
		if err != nil {
			return fmt.Errorf("scanning source archive: %v", err)
		}
	}

	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("opening source archive: %v", err)
	}
	defer file.Close()

	err = t.Open(file, 0)
	if err != nil {
		return fmt.Errorf("opening tar archive for reading: %v", err)
	}
	defer t.Close()

	for {
		err := t.untarNext(destination)
		if err == io.EOF {
			break
		}
		if err != nil {
			if t.ContinueOnError || IsIllegalPathError(err) {
				log.Printf("[ERROR] Reading file in tar archive: %v", err)
				continue
			}
			return fmt.Errorf("reading file in tar archive: %v", err)
		}
	}

	return nil
}

// addTopLevelFolder scans the files contained inside
// the tarball named sourceArchive and returns a modified
// destination if all the files do not share the same
// top-level folder.
func (t *Tar) addTopLevelFolder(sourceArchive, destination string) (string, error) {
	file, err := os.Open(sourceArchive)
	if err != nil {
		return "", fmt.Errorf("opening source archive: %v", err)
	}
	defer file.Close()

	// if the reader is to be wrapped, ensure we do that now
	// or we will not be able to read the archive successfully
	reader := io.Reader(file)
	if t.readerWrapFn != nil {
		reader, err = t.readerWrapFn(reader)
		if err != nil {
			return "", fmt.Errorf("wrapping reader: %v", err)
		}
	}
	if t.cleanupWrapFn != nil {
		defer t.cleanupWrapFn()
	}

	tr := tar.NewReader(reader)

	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("scanning tarball's file listing: %v", err)
		}
		files = append(files, hdr.Name)
	}

	if multipleTopLevels(files) {
		destination = filepath.Join(destination, folderNameFromFileName(sourceArchive))
	}

	return destination, nil
}

func (t *Tar) untarNext(destination string) error {
	f, err := t.Read()
	if err != nil {
		return err // don't wrap error; calling loop must break on io.EOF
	}
	defer f.Close()

	header, ok := f.Header.(*tar.Header)
	if !ok {
		return fmt.Errorf("expected header to be *tar.Header but was %T", f.Header)
	}

	errPath := t.CheckPath(destination, header.Name)
	if errPath != nil {
		return fmt.Errorf("checking path traversal attempt: %v", errPath)
	}

	if t.StripComponents > 0 {
		if strings.Count(header.Name, "/") < t.StripComponents {
			return nil // skip path with fewer components
		}

		for i := 0; i < t.StripComponents; i++ {
			slash := strings.Index(header.Name, "/")
			header.Name = header.Name[slash+1:]
		}
	}
	return t.untarFile(f, destination, header)
}

func (t *Tar) untarFile(f File, destination string, hdr *tar.Header) error {
	to := filepath.Join(destination, hdr.Name)

	// do not overwrite existing files, if configured
	if !f.IsDir() && !t.OverwriteExisting && fileExists(to) {
		return fmt.Errorf("file already exists: %s", to)
	}

	switch hdr.Typeflag {
	case tar.TypeDir:
		return mkdir(to, f.Mode())
	case tar.TypeReg, tar.TypeRegA, tar.TypeChar, tar.TypeBlock, tar.TypeFifo, tar.TypeGNUSparse:
		return writeNewFile(to, f, f.Mode())
	case tar.TypeSymlink:
		return writeNewSymbolicLink(to, hdr.Linkname)
	case tar.TypeLink:
		return writeNewHardLink(to, filepath.Join(destination, hdr.Linkname))
	case tar.TypeXGlobalHeader:
		return nil // ignore the pax global header from git-generated tarballs
	default:
		return fmt.Errorf("%s: unknown type flag: %c", hdr.Name, hdr.Typeflag)
	}
}

func (t *Tar) writeWalk(source, topLevelFolder, destination string) error {
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
			if t.ContinueOnError {
				log.Printf("[ERROR] Walking %s: %v", fpath, err)
				return nil
			}
			return err
		}
		if err != nil {
			return handleErr(fmt.Errorf("traversing %s: %v", fpath, err))
		}
		if info == nil {
			return handleErr(fmt.Errorf("no file info"))
		}

		// make sure we do not copy our output file into itself
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
		err = t.Write(File{
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

// Create opens t for writing a tar archive to out.
func (t *Tar) Create(out io.Writer) error {
	if t.tw != nil {
		return fmt.Errorf("tar archive is already created for writing")
	}

	// wrapping writers allows us to output
	// compressed tarballs, for example
	if t.writerWrapFn != nil {
		var err error
		out, err = t.writerWrapFn(out)
		if err != nil {
			return fmt.Errorf("wrapping writer: %v", err)
		}
	}

	t.tw = tar.NewWriter(out)
	return nil
}

// Write writes f to t, which must have been opened for writing first.
func (t *Tar) Write(f File) error {
	if t.tw == nil {
		return fmt.Errorf("tar archive was not created for writing first")
	}
	if f.FileInfo == nil {
		return fmt.Errorf("no file info")
	}
	if f.FileInfo.Name() == "" {
		return fmt.Errorf("missing file name")
	}

	var linkTarget string
	if isSymlink(f) {
		var err error
		linkTarget, err = os.Readlink(f.Name())
		if err != nil {
			return fmt.Errorf("%s: readlink: %v", f.Name(), err)
		}
	}

	hdr, err := tar.FileInfoHeader(f, filepath.ToSlash(linkTarget))
	if err != nil {
		return fmt.Errorf("%s: making header: %v", f.Name(), err)
	}

	err = t.tw.WriteHeader(hdr)
	if err != nil {
		return fmt.Errorf("%s: writing header: %v", hdr.Name, err)
	}

	if f.IsDir() {
		return nil // directories have no contents
	}

	if hdr.Typeflag == tar.TypeReg {
		if f.ReadCloser == nil {
			return fmt.Errorf("%s: no way to read file contents", f.Name())
		}
		_, err := io.Copy(t.tw, f)
		if err != nil {
			return fmt.Errorf("%s: copying contents: %v", f.Name(), err)
		}
	}

	return nil
}

// Open opens t for reading an archive from
// in. The size parameter is not used.
func (t *Tar) Open(in io.Reader, size int64) error {
	if t.tr != nil {
		return fmt.Errorf("tar archive is already open for reading")
	}
	// wrapping readers allows us to open compressed tarballs
	if t.readerWrapFn != nil {
		var err error
		in, err = t.readerWrapFn(in)
		if err != nil {
			return fmt.Errorf("wrapping file reader: %v", err)
		}
	}
	t.tr = tar.NewReader(in)
	return nil
}

// Read reads the next file from t, which must have
// already been opened for reading. If there are no
// more files, the error is io.EOF. The File must
// be closed when finished reading from it.
func (t *Tar) Read() (File, error) {
	if t.tr == nil {
		return File{}, fmt.Errorf("tar archive is not open")
	}

	hdr, err := t.tr.Next()
	if err != nil {
		return File{}, err // don't wrap error; preserve io.EOF
	}

	file := File{
		FileInfo:   hdr.FileInfo(),
		Header:     hdr,
		ReadCloser: ReadFakeCloser{t.tr},
	}

	return file, nil
}

// Close closes the tar archive(s) opened by Create and Open.
func (t *Tar) Close() error {
	var err error
	if t.tr != nil {
		t.tr = nil
	}
	if t.tw != nil {
		tw := t.tw
		t.tw = nil
		err = tw.Close()
	}
	// make sure cleanup of "Reader/Writer wrapper"
	// (say that ten times fast) happens AFTER the
	// underlying stream is closed
	if t.cleanupWrapFn != nil {
		t.cleanupWrapFn()
	}
	return err
}

// Walk calls walkFn for each visited item in archive.
func (t *Tar) Walk(archive string, walkFn WalkFunc) error {
	file, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("opening archive file: %v", err)
	}
	defer file.Close()

	err = t.Open(file, 0)
	if err != nil {
		return fmt.Errorf("opening archive: %v", err)
	}
	defer t.Close()

	for {
		f, err := t.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			if t.ContinueOnError {
				log.Printf("[ERROR] Opening next file: %v", err)
				continue
			}
			return fmt.Errorf("opening next file: %v", err)
		}
		err = walkFn(f)
		if err != nil {
			if err == ErrStopWalk {
				break
			}
			if t.ContinueOnError {
				log.Printf("[ERROR] Walking %s: %v", f.Name(), err)
				continue
			}
			return fmt.Errorf("walking %s: %v", f.Name(), err)
		}
	}

	return nil
}

// Extract extracts a single file from the tar archive.
// If the target is a directory, the entire folder will
// be extracted into destination.
func (t *Tar) Extract(source, target, destination string) error {
	// target refers to a path inside the archive, which should be clean also
	target = path.Clean(target)

	// if the target ends up being a directory, then
	// we will continue walking and extracting files
	// until we are no longer within that directory
	var targetDirPath string

	return t.Walk(source, func(f File) error {
		th, ok := f.Header.(*tar.Header)
		if !ok {
			return fmt.Errorf("expected header to be *tar.Header but was %T", f.Header)
		}

		// importantly, cleaning the path strips tailing slash,
		// which must be appended to folders within the archive
		name := path.Clean(th.Name)
		if f.IsDir() && target == name {
			targetDirPath = path.Dir(name)
		}

		if within(target, th.Name) {
			// either this is the exact file we want, or is
			// in the directory we want to extract

			// build the filename we will extract to
			end, err := filepath.Rel(targetDirPath, th.Name)
			if err != nil {
				return fmt.Errorf("relativizing paths: %v", err)
			}
			th.Name = end

			// relativize any hardlink names
			if th.Typeflag == tar.TypeLink {
				th.Linkname = filepath.Join(filepath.Base(filepath.Dir(th.Linkname)), filepath.Base(th.Linkname))
			}

			err = t.untarFile(f, destination, th)
			if err != nil {
				return fmt.Errorf("extracting file %s: %v", th.Name, err)
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
func (*Tar) Match(file io.ReadSeeker) (bool, error) {
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

	buf := make([]byte, tarBlockSize)
	if _, err = io.ReadFull(file, buf); err != nil {
		return false, nil
	}
	return hasTarHeader(buf), nil
}

// hasTarHeader checks passed bytes has a valid tar header or not. buf must
// contain at least 512 bytes and if not, it always returns false.
func hasTarHeader(buf []byte) bool {
	if len(buf) < tarBlockSize {
		return false
	}

	b := buf[148:156]
	b = bytes.Trim(b, " \x00") // clean up all spaces and null bytes
	if len(b) == 0 {
		return false // unknown format
	}
	hdrSum, err := strconv.ParseUint(string(b), 8, 64)
	if err != nil {
		return false
	}

	// According to the go official archive/tar, Sun tar uses signed byte
	// values so this calcs both signed and unsigned
	var usum uint64
	var sum int64
	for i, c := range buf {
		if 148 <= i && i < 156 {
			c = ' ' // checksum field itself is counted as branks
		}
		usum += uint64(uint8(c))
		sum += int64(int8(c))
	}

	if hdrSum != usum && int64(hdrSum) != sum {
		return false // invalid checksum
	}

	return true
}

func (t *Tar) String() string { return "tar" }

// NewTar returns a new, default instance ready to be customized and used.
func NewTar() *Tar {
	return &Tar{
		MkdirAll: true,
	}
}

const tarBlockSize = 512

// Compile-time checks to ensure type implements desired interfaces.
var (
	_ = Reader(new(Tar))
	_ = Writer(new(Tar))
	_ = Archiver(new(Tar))
	_ = Unarchiver(new(Tar))
	_ = Walker(new(Tar))
	_ = Extractor(new(Tar))
	_ = Matcher(new(Tar))
	_ = ExtensionChecker(new(Tar))
	_ = FilenameChecker(new(Tar))
)

// DefaultTar is a default instance that is conveniently ready to use.
var DefaultTar = NewTar()
