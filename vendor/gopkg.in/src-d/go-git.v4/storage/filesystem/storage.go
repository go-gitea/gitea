// Package filesystem is a storage backend base on filesystems
package filesystem

import (
	"gopkg.in/src-d/go-git.v4/storage/filesystem/internal/dotgit"

	"srcd.works/go-billy.v1"
)

// Storage is an implementation of git.Storer that stores data on disk in the
// standard git format (this is, the .git directory). Zero values of this type
// are not safe to use, see the NewStorage function below.
type Storage struct {
	ObjectStorage
	ReferenceStorage
	ShallowStorage
	ConfigStorage
}

// NewStorage returns a new Storage backed by a given `fs.Filesystem`
func NewStorage(fs billy.Filesystem) (*Storage, error) {
	dir := dotgit.New(fs)
	o, err := newObjectStorage(dir)
	if err != nil {
		return nil, err
	}

	return &Storage{
		ObjectStorage:    o,
		ReferenceStorage: ReferenceStorage{dir: dir},
		ShallowStorage:   ShallowStorage{dir: dir},
		ConfigStorage:    ConfigStorage{dir: dir},
	}, nil
}
