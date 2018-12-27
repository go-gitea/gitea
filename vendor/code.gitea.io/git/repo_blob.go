// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

func (repo *Repository) getBlob(id SHA1) (*Blob, error) {
	if _, err := NewCommand("cat-file", "-p", id.String()).RunInDir(repo.Path); err != nil {
		return nil, ErrNotExist{id.String(), ""}
	}

	return &Blob{
		repo: repo,
		TreeEntry: &TreeEntry{
			ID: id,
			ptree: &Tree{
				repo: repo,
			},
		},
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
