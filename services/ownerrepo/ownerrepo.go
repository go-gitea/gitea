// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ownerrepo

import (
	"slices"

	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/reqctx"
)

var DefaultsRepoNames = []string{".gitea", ".github"}

type Repo struct {
	Repo    *repo_model.Repository
	GitRepo *git.Repository
	Commit  *git.Commit
}

func Find(ctx reqctx.RequestContext, doer, owner *user_model.User, names []string, match func(*Repo) bool) *Repo { // see docs/design/owner-repositories.md
	repos, err := repo_model.GetOwnerRepositoriesByNames(ctx, owner.ID, names)
	if err != nil {
		log.Error("ownerrepo.Find: %v", err)
		return nil
	}
	for _, name := range names {
		idx := slices.IndexFunc(repos, func(repo *repo_model.Repository) bool { return repo.LowerName == name })
		if idx == -1 {
			continue
		}
		if found := open(ctx, doer, owner, repos[idx]); found != nil && match(found) {
			return found
		}
	}
	return nil
}

func open(ctx reqctx.RequestContext, doer, owner *user_model.User, repo *repo_model.Repository) *Repo {
	if repo.IsEmpty {
		return nil
	}
	repo.Owner = owner
	perm, err := access_model.GetDoerRepoPermission(ctx, repo, doer)
	if err != nil {
		log.Error("ownerrepo.open %s: %v", repo.FullName(), err)
		return nil
	}
	if !perm.CanRead(unit.TypeCode) {
		return nil
	}
	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		log.Error("ownerrepo.open %s: %v", repo.FullName(), err)
		return nil
	}
	commit, err := gitRepo.GetBranchCommit(ctx, repo.DefaultBranch)
	if err != nil {
		log.Error("ownerrepo.open %s: %v", repo.FullName(), err)
		return nil
	}
	return &Repo{Repo: repo, GitRepo: gitRepo, Commit: commit}
}
