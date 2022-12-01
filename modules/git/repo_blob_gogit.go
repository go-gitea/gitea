// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"github.com/go-git/go-git/v5/plumbing"
)

func (repo *Repository) getBlob(id SHA1) (*Blob, error) {
	encodedObj, err := repo.gogitRepo.Storer.EncodedObject(plumbing.AnyObject, id)
	if err != nil {
		return nil, ErrNotExist{id.String(), ""}
	}

	return &Blob{
		ID:              id,
		gogitEncodedObj: encodedObj,
	}, nil
}
