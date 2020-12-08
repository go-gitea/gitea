// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"context"
	"io"
)

// GitService represents a complete git service
type GitService interface {
	RepositoryService
	ArchiveService
	CommitsInfoService
	AttributeService
	LogService
	IndexService
	BlameService
	NoteService
	HashService
}

// RepositoryService represents a service that creates and opens repositories
type RepositoryService interface {
	OpenRepository(path string) (Repository, error)
}

//
//  /\  ._  _ |_  o     _
// /--\ |  (_ | | | \/ (/_
//

// ArchiveService represents a service that creates archives from repositories
type ArchiveService interface {
	CreateArchive(ctx context.Context, repository Repository, treeish, filename string, opts CreateArchiveOpts) error
}

// ArchiveType archive types
type ArchiveType int

const (
	// ZIP zip archive type
	ZIP ArchiveType = iota + 1
	// TARGZ tar gz archive type
	TARGZ
)

// String converts an ArchiveType to string
func (a ArchiveType) String() string {
	switch a {
	case ZIP:
		return "zip"
	case TARGZ:
		return "tar.gz"
	}
	return "unknown"
}

// CreateArchiveOpts represents options for creating an archive
type CreateArchiveOpts struct {
	Format ArchiveType
	Prefix bool
}

//
//  /\  _|_ _|_ ._ o |_      _|_  _
// /--\  |_  |_ |  | |_) |_|  |_ (/_
//

// AttributeService represeents a service that provides attributes
type AttributeService interface {
	// CheckAttribute returns an attribute map
	CheckAttribute(repo Repository, opts CheckAttributeOpts) (map[string]map[string]string, error)
}

// CheckAttributeOpts represents the possible options to CheckAttribute
type CheckAttributeOpts struct {
	CachedOnly    bool
	AllAttributes bool
	Attributes    []string
	Filenames     []string
}

//
// |_|  _.  _ |_
// | | (_| _> | |
//

// HashService provides functions for generating hashes
type HashService interface {

	// ComputeHash compute the hash for a given ObjectType and reader
	ComputeHash(typ ObjectType, size int64, reader io.Reader) (Hash, error)

	// ComputeBlobHash computes the hash of a blob
	ComputeBlobHash(size int64, reader io.Reader) (Hash, error)

	// MustHashFromString converts a string to a hash
	MustHashFromString(sha string) Hash
}
