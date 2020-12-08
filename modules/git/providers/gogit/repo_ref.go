// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"strings"

	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// GetRefsFiltered returns all references of the repository that matches patterm exactly or starting with.
func (repo *Repository) GetRefsFiltered(pattern string) ([]service.Reference, error) {
	r, err := gogit.PlainOpen(repo.Path())
	if err != nil {
		return nil, err
	}

	refsIter, err := r.References()
	if err != nil {
		return nil, err
	}
	refs := make([]service.Reference, 0)
	if err = refsIter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name() != plumbing.HEAD && !ref.Name().IsRemote() &&
			(pattern == "" || strings.HasPrefix(ref.Name().String(), pattern)) {
			refType := service.ObjectCommit
			if ref.Name().IsTag() {
				// tags can be of type `commit` (lightweight) or `tag` (annotated)
				if tagType, _ := repo.GetTagType(fromPlumbingHash(ref.Hash())); err == nil {
					refType = service.ObjectType(tagType)
				}
			}
			r := common.NewReference(
				ref.Name().String(),
				fromPlumbingHash(ref.Hash()),
				refType,
				repo,
			)
			refs = append(refs, r)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return refs, nil
}

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]service.Reference, error) {
	return repo.GetRefsFiltered("")
}
