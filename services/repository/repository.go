// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unit"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
)

// Init start repository service
func Init() error {
	repo_module.LoadRepoConfig()
	system_model.RemoveAllWithNotice(db.DefaultContext, "Clean up temporary repository uploads", setting.Repository.Upload.TempPath)
	system_model.RemoveAllWithNotice(db.DefaultContext, "Clean up temporary repositories", repo_module.LocalCopyPath())
	return initPushQueue()
}

// UpdateRepository updates a repository
func UpdateRepository(repo *repo_model.Repository, visibilityChanged bool) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = repo_module.UpdateRepository(ctx, repo, visibilityChanged); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	return committer.Commit()
}

// LinkedRepository returns the linked repo if any
func LinkedRepository(ctx context.Context, a *repo_model.Attachment) (*repo_model.Repository, unit.Type, error) {
	if a.IssueID != 0 {
		iss, err := issues_model.GetIssueByID(ctx, a.IssueID)
		if err != nil {
			return nil, unit.TypeIssues, err
		}
		repo, err := repo_model.GetRepositoryByID(ctx, iss.RepoID)
		unitType := unit.TypeIssues
		if iss.IsPull {
			unitType = unit.TypePullRequests
		}
		return repo, unitType, err
	} else if a.ReleaseID != 0 {
		rel, err := repo_model.GetReleaseByID(ctx, a.ReleaseID)
		if err != nil {
			return nil, unit.TypeReleases, err
		}
		repo, err := repo_model.GetRepositoryByID(ctx, rel.RepoID)
		return repo, unit.TypeReleases, err
	}
	return nil, -1, nil
}
