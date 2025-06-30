// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"github.com/go-git/go-git/v5/plumbing"
)

func (repo *Repository) getBlob(id ObjectID) (*Blob, error) {
	encodedObj, err := repo.gogitRepo.Storer.EncodedObject(plumbing.AnyObject, plumbing.Hash(id.RawValue()))
	if err != nil {
		return nil, ErrNotExist{id.String(), ""}
	}

	return &Blob{
		ID:              id,
		gogitEncodedObj: encodedObj,
	}, nil
}
