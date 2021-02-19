// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package lfs

import (
	"code.gitea.io/gitea/modules/git"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// SearchPointerFiles scans the whole repository for LFS pointer files
func SearchPointerFiles(repo *git.Repository) ([]*Pointer, error) {
	gitRepo := repo.GoGitRepo()

	blobs, err := gitRepo.BlobObjects()
	if err != nil {
		return nil, err
	}

	var pointers []*Pointer

	err = blobs.ForEach(func(blob *object.Blob) error {
		if blob.Size > blobSizeCutoff {
			return nil
		}

		reader, err := blob.Reader()
		if err != nil {
			return nil
		}
		defer reader.Close()

		pointer := TryReadPointer(reader)
		if pointer != nil {
			pointers = append(pointers, pointer)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return pointers, nil
}