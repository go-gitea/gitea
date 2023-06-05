// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]*Reference, error) {
	return repo.GetRefsFiltered("")
}
