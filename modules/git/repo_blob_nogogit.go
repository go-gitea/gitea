// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

func (repo *Repository) getBlob(id SHA1) (*Blob, error) {
	if id.IsZero() {
		return nil, ErrNotExist{id.String(), ""}
	}
	return &Blob{
		ID:   id,
		repo: repo,
	}, nil
}
