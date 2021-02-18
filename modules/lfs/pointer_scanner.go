// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package lfs

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"

	"github.com/go-git/go-git/v5/plumbing/object"
)

const blobSizeCutoff = 1024

// TODO Combine with methods in pointers.go
// TryReadPointer will return a pointer if the provided byte slice is a pointer file or nil otherwise.
func TryReadPointer(buf []byte) *Pointer {
	headString := string(buf)
	if !strings.HasPrefix(headString, models.LFSMetaFileIdentifier) {
		return nil
	}

	splitLines := strings.Split(headString, "\n")
	if len(splitLines) < 3 {
		return nil
	}

	oid := strings.TrimPrefix(splitLines[1], models.LFSMetaFileOidPrefix)
	size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
	if len(oid) != 64 || err != nil {
		return nil
	}

	return &Pointer{Oid: oid, Size: size}
}

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

		buf := make([]byte, blobSizeCutoff)
		n, _ := reader.Read(buf)
		buf = buf[:n]

		pointer := TryReadPointer(buf)
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