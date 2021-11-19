// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build gogit
// +build gogit

package lfs

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/git"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// SearchPointerBlobs scans the whole repository for LFS pointer files
func SearchPointerBlobs(ctx context.Context, repo *git.Repository, pointerChan chan<- PointerBlob, errChan chan<- error) {
	gitRepo := repo.GoGitRepo()

	err := func() error {
		blobs, err := gitRepo.BlobObjects()
		if err != nil {
			return fmt.Errorf("lfs.SearchPointerBlobs BlobObjects: %w", err)
		}

		return blobs.ForEach(func(blob *object.Blob) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

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
				pointerChan <- PointerBlob{Hash: blob.Hash.String(), Pointer: pointer}
			}

			return nil
		})
	}()

	if err != nil {
		select {
		case <-ctx.Done():
		default:
			errChan <- err
		}
	}

	close(pointerChan)
	close(errChan)
}
