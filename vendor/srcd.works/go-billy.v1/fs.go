// FS abstraction interfaces
package billy

import (
	"errors"
	"io"
	"os"
)

var (
	ErrClosed       = errors.New("file: Writing on closed file.")
	ErrReadOnly     = errors.New("this is a read-only filesystem")
	ErrNotSupported = errors.New("feature not supported")
)

// Filesystem abstract the operations in a storage-agnostic interface.
// It allows you to:
// * Create files.
// * Open existing files.
// * Get info about files.
// * List files in a directory.
// * Get a temporal file.
// * Rename files.
// * Remove files.
// * Join parts of path.
// * Obtain a filesystem starting on a subdirectory in the current filesystem.
// * Get the base path for the filesystem.
// Each method implementation varies from implementation to implementation. Refer to
// the specific documentation for more info.
type Filesystem interface {
	Create(filename string) (File, error)
	Open(filename string) (File, error)
	OpenFile(filename string, flag int, perm os.FileMode) (File, error)
	Stat(filename string) (FileInfo, error)
	ReadDir(path string) ([]FileInfo, error)
	TempFile(dir, prefix string) (File, error)
	Rename(from, to string) error
	Remove(filename string) error
	Join(elem ...string) string
	Dir(path string) Filesystem
	Base() string
}

// File implements io.Closer, io.Reader, io.Seeker, and io.Writer>
// Provides method to obtain the file name and the state of the file (open or closed).
type File interface {
	Filename() string
	IsClosed() bool
	io.Writer
	io.Reader
	io.Seeker
	io.Closer
}

type FileInfo os.FileInfo

type BaseFile struct {
	BaseFilename string
	Closed       bool
}

func (f *BaseFile) Filename() string {
	return f.BaseFilename
}

func (f *BaseFile) IsClosed() bool {
	return f.Closed
}
