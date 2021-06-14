// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	if name == "" {
		return false
	}

	return repo.IsReferenceExist(TagPrefix + name)
}

// GetTags returns all tags of the repository.
func (repo *Repository) GetTags() (tags []string, err error) {
	tags, _, err = callShowRef(repo.Path, TagPrefix, "--tags", 0, 0)
	return
}
