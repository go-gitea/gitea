// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

// GetBlob finds the blob object in the repository.
func (repo *Repository) GetBlob(idStr string) (*Blob, error) {
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	if id.IsZero() {
		return nil, ErrNotExist{id.String(), ""}
	}
	return &Blob{
		ID:   id,
		repo: repo,
	}, nil
}
