// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

func (repo *Repository) getBlob(id SHA1) (*Blob, error) {
	if id.IsZero() {
		return nil, ErrNotExist{id.String(), ""}
	}
	return &Blob{
		ID:       id,
		repoPath: repo.Path,
	}, nil
}
