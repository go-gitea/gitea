// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"gopkg.in/src-d/go-git.v4/plumbing"
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

// GetBlob finds the blob object in the repository.
func (repo *Repository) GetBlob(idStr string) (*Blob, error) {
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	return repo.getBlob(id)
}
