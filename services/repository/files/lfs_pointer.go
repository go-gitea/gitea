// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"io"

	git_model "gitea.dev/models/git"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/util"
)

// ErrLFSPointerObjectMissing is a write that claims to carry an LFS pointer
// for an object this repository does not hold. Committing it would put a file
// in the tree that no clone can resolve: git-lfs would ask for the object and
// be told it is not there.
type ErrLFSPointerObjectMissing struct {
	Path string
	Oid  string
}

// IsErrLFSPointerObjectMissing checks if an error is an ErrLFSPointerObjectMissing.
func IsErrLFSPointerObjectMissing(err error) bool {
	_, ok := err.(ErrLFSPointerObjectMissing)
	return ok
}

func (err ErrLFSPointerObjectMissing) Error() string {
	return fmt.Sprintf("repository holds no LFS object for this pointer [path: %s, oid: %s]", err.Path, err.Oid)
}

func (err ErrLFSPointerObjectMissing) Unwrap() error {
	// Invalid rather than not-found: the request is well formed, and what
	// makes it unprocessable is the repository's state, not a bad route.
	return util.ErrInvalidArgument
}

// verifyLFSPointerIsResolvable reads the pointer a caller says it is
// committing and confirms this repository can actually serve the object it
// names: the metadata row that makes the object this repository's, and the
// bytes themselves in the content store. The reader is rewound, so the caller
// writes the same content it would have written anyway.
func verifyLFSPointerIsResolvable(ctx context.Context, file *ChangeRepoFile, contentStore *lfs.ContentStore, repoID int64) error {
	content, err := io.ReadAll(io.LimitReader(file.ContentReader, lfs.MetaFileMaxSize))
	if err != nil {
		return fmt.Errorf("read claimed LFS pointer: %w", err)
	}
	if _, err := file.ContentReader.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind claimed LFS pointer: %w", err)
	}
	pointer, err := lfs.ReadPointerFromBuffer(content)
	if err != nil {
		// Not a pointer at all. Writing it as-is would commit whatever this
		// is to a path the repository routes through LFS.
		return ErrLFSPointerObjectMissing{Path: file.TreePath}
	}
	if _, err := git_model.GetLFSMetaObjectByOid(ctx, repoID, pointer.Oid); err != nil {
		// The object may well exist for some other repository; it is not this
		// one's, and a clone of this one could not fetch it.
		return ErrLFSPointerObjectMissing{Path: file.TreePath, Oid: pointer.Oid}
	}
	exists, err := contentStore.Exists(pointer)
	if err != nil {
		return fmt.Errorf("look up LFS object %s: %w", pointer.Oid, err)
	}
	if !exists {
		// A metadata row with no bytes behind it: the batch endpoint records
		// the intention to upload before the upload happens.
		return ErrLFSPointerObjectMissing{Path: file.TreePath, Oid: pointer.Oid}
	}
	return nil
}
