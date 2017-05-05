// Provides a billy filesystem for the OS.
package os

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"srcd.works/go-billy.v1"
)

// OS is a filesystem based on the os filesystem
type OS struct {
	base string
}

// New returns a new OS filesystem
func New(baseDir string) *OS {
	return &OS{
		base: baseDir,
	}
}

// Create creates a file and opens it with standard permissions
// and modes O_RDWR, O_CREATE and O_TRUNC.
func (fs *OS) Create(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// OpenFile is equivalent to standard os.OpenFile.
// If flag os.O_CREATE is set, all parent directories will be created.
func (fs *OS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	fullpath := path.Join(fs.base, filename)

	if flag&os.O_CREATE != 0 {
		if err := fs.createDir(fullpath); err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(fullpath, flag, perm)
	if err != nil {
		return nil, err
	}

	filename, err = filepath.Rel(fs.base, fullpath)
	if err != nil {
		return nil, err
	}

	return newOSFile(filename, f), nil
}

func (fs *OS) createDir(fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// ReadDir returns the filesystem info for all the archives under the specified
// path.
func (ofs *OS) ReadDir(path string) ([]billy.FileInfo, error) {
	fullpath := ofs.Join(ofs.base, path)

	l, err := ioutil.ReadDir(fullpath)
	if err != nil {
		return nil, err
	}

	var s = make([]billy.FileInfo, len(l))
	for i, f := range l {
		s[i] = f
	}

	return s, nil
}

// Rename moves a file in disk from _from_ to _to_.
func (fs *OS) Rename(from, to string) error {
	from = fs.Join(fs.base, from)
	to = fs.Join(fs.base, to)

	if err := fs.createDir(to); err != nil {
		return err
	}

	return os.Rename(from, to)
}

// Open opens a file in read-only mode.
func (fs *OS) Open(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

// Stat returns the FileInfo structure describing file.
func (fs *OS) Stat(filename string) (billy.FileInfo, error) {
	fullpath := fs.Join(fs.base, filename)
	return os.Stat(fullpath)
}

// Remove deletes a file in disk.
func (fs *OS) Remove(filename string) error {
	fullpath := fs.Join(fs.base, filename)
	return os.Remove(fullpath)
}

// TempFile creates a new temporal file.
func (fs *OS) TempFile(dir, prefix string) (billy.File, error) {
	fullpath := fs.Join(fs.base, dir)
	if err := fs.createDir(fullpath + string(os.PathSeparator)); err != nil {
		return nil, err
	}

	f, err := ioutil.TempFile(fullpath, prefix)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		return nil, err
	}

	filename, err := filepath.Rel(fs.base, fs.Join(fullpath, s.Name()))
	if err != nil {
		return nil, err
	}

	return newOSFile(filename, f), nil
}

// Join joins the specified elements using the filesystem separator.
func (fs *OS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Dir returns a new Filesystem from the same type of fs using as baseDir the
// given path
func (fs *OS) Dir(path string) billy.Filesystem {
	return New(fs.Join(fs.base, path))
}

// Base returns the base path of the filesytem
func (fs *OS) Base() string {
	return fs.base
}

// osFile represents a file in the os filesystem
type osFile struct {
	billy.BaseFile
	file *os.File
}

func newOSFile(filename string, file *os.File) billy.File {
	return &osFile{
		BaseFile: billy.BaseFile{BaseFilename: filename},
		file:     file,
	}
}

func (f *osFile) Read(p []byte) (int, error) {
	return f.file.Read(p)
}

func (f *osFile) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

func (f *osFile) Write(p []byte) (int, error) {
	return f.file.Write(p)
}

func (f *osFile) Close() error {
	f.BaseFile.Closed = true

	return f.file.Close()
}

func (f *osFile) ReadAt(p []byte, off int64) (int, error) {
	return f.file.ReadAt(p, off)
}
