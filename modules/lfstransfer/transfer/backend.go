package transfer

import (
	"io"
)

const (
	// UploadOperation is an upload operation.
	UploadOperation = "upload"
	// DownloadOperation is a download operation.
	DownloadOperation = "download"
)

// Backend is a Git LFS backend.
type Backend interface {
	Batch(op string, pointers []BatchItem, args Args) ([]BatchItem, error)
	Upload(oid string, size int64, r io.Reader, args Args) error
	Verify(oid string, size int64, args Args) (Status, error)
	Download(oid string, args Args) (io.ReadCloser, int64, error)
	LockBackend(args Args) LockBackend
}

// Lock is a Git LFS lock.
type Lock interface {
	Unlock() error
	ID() string
	Path() string
	FormattedTimestamp() string
	OwnerName() string
	AsLockSpec(ownerID bool) ([]string, error)
	AsArguments() []string
}

// LockBackend is a Git LFS lock backend.
type LockBackend interface {
	// Create creates a lock for the given path and refname.
	// Refname can be empty.
	Create(path, refname string) (Lock, error)
	Unlock(lock Lock) error
	FromPath(path string) (Lock, error)
	FromID(id string) (Lock, error)
	Range(cursor string, limit int, iter func(Lock) error) (string, error)
}
