// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]*Reference, error) {
	return repo.GetRefsFiltered("")
}

// GetRefsFiltered returns all references of the repository that matches patterm exactly or starting with.
func (repo *Repository) GetRefsFiltered(pattern string) ([]*Reference, error) {
	r, err := git.PlainOpen(repo.Path)
	if err != nil {
		return nil, err
	}

	refsIter, err := r.References()
	if err != nil {
		return nil, err
	}
	refs := make([]*Reference, 0)
	if err = refsIter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name() != plumbing.HEAD && !ref.Name().IsRemote() &&
			(pattern == "" || strings.HasPrefix(ref.Name().String(), pattern)) {
			refType := string(ObjectCommit)
			if ref.Name().IsTag() {
				// tags can be of type `commit` (lightweight) or `tag` (annotated)
				if tagType, _ := repo.GetTagType(ref.Hash()); err == nil {
					refType = tagType
				}
			}
			r := &Reference{
				Name:   ref.Name().String(),
				Object: ref.Hash(),
				Type:   refType,
				repo:   repo,
			}
			refs = append(refs, r)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return refs, nil
}
