// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build gogit
// +build gogit

package git

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	_, err := repo.gogitRepo.Reference(plumbing.ReferenceName(TagPrefix+name), true)
	return err == nil
}

// GetTags returns all tags of the repository.
func (repo *Repository) GetTags() ([]string, error) {
	var tagNames []string

	tags, err := repo.gogitRepo.Tags()
	if err != nil {
		return nil, err
	}

	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		tagNames = append(tagNames, strings.TrimPrefix(tag.Name().String(), TagPrefix))
		return nil
	})

	// Reverse order
	for i := 0; i < len(tagNames)/2; i++ {
		j := len(tagNames) - i - 1
		tagNames[i], tagNames[j] = tagNames[j], tagNames[i]
	}

	return tagNames, nil
}
