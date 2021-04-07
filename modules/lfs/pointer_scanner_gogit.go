// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package lfs

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// SearchPointerBlobs scans the whole repository for LFS pointer files
func SearchPointerBlobs(repo *git.Repository) ([]PointerBlob, error) {
	gitRepo := repo.GoGitRepo()

	blobs, err := gitRepo.BlobObjects()
	if err != nil {
		return nil, fmt.Errorf("lfs.SearchPointerBlobs BlobObjects: %w", err)
	}

	var pointers []PointerBlob

	err = blobs.ForEach(func(blob *object.Blob) error {
		if blob.Size > blobSizeCutoff {
			return nil
		}

		reader, err := blob.Reader()
		if err != nil {
			return fmt.Errorf("lfs.SearchPointerBlobs blob.Reader: %w", err)
		}
		defer reader.Close()

		pointer, _ := ReadPointer(reader)
		if pointer.IsValid() {
			pointers = append(pointers, PointerBlob{Hash: blob.Hash.String(), Pointer: pointer})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return pointers, nil
}
