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
			r := &Reference{
				Name:   ref.Name().String(),
				Object: SHA1(ref.Hash()),
				Type:   string(ObjectCommit),
				repo:   repo,
			}
			if ref.Name().IsTag() {
				r.Type = string(ObjectTag)
			}
			refs = append(refs, r)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return refs, nil
}
