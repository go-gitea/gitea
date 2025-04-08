// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

// GenerateIssueLabels generates issue labels from a template repository
func GenerateIssueLabels(ctx context.Context, templateRepo, generateRepo *repo_model.Repository) error {
	templateLabels, err := issues_model.GetLabelsByRepoID(ctx, templateRepo.ID, "", db.ListOptions{})
	if err != nil {
		return err
	}
	// Prevent insert being called with an empty slice which would result in
	// err "no element on slice when insert".
	if len(templateLabels) == 0 {
		return nil
	}

	newLabels := make([]*issues_model.Label, 0, len(templateLabels))
	for _, templateLabel := range templateLabels {
		newLabels = append(newLabels, &issues_model.Label{
			RepoID:      generateRepo.ID,
			Name:        templateLabel.Name,
			Exclusive:   templateLabel.Exclusive,
			Description: templateLabel.Description,
			Color:       templateLabel.Color,
		})
	}
	return db.Insert(ctx, newLabels)
}

func GenerateProtectedBranch(ctx context.Context, templateRepo, generateRepo *repo_model.Repository) error {
	templateBranches, err := git_model.FindRepoProtectedBranchRules(ctx, templateRepo.ID)
	if err != nil {
		return err
	}
	// Prevent insert being called with an empty slice which would result in
	// err "no element on slice when insert".
	if len(templateBranches) == 0 {
		return nil
	}

	newBranches := make([]*git_model.ProtectedBranch, 0, len(templateBranches))
	for _, templateBranch := range templateBranches {
		templateBranch.ID = 0
		templateBranch.RepoID = generateRepo.ID
		templateBranch.UpdatedUnix = 0
		templateBranch.CreatedUnix = 0
		newBranches = append(newBranches, templateBranch)
	}
	return db.Insert(ctx, newBranches)
}

// GenerateRepository generates a repository from a template
func GenerateRepository(ctx context.Context, doer, owner *user_model.User, templateRepo *repo_model.Repository, opts GenerateRepoOptions) (_ *repo_model.Repository, err error) {
	if !doer.CanCreateRepoIn(owner) {
		return nil, repo_model.ErrReachLimitOfRepo{
			Limit: owner.MaxRepoCreation,
		}
	}

	generateRepo := &repo_model.Repository{
		OwnerID:          owner.ID,
		Owner:            owner,
		OwnerName:        owner.Name,
		Name:             opts.Name,
		LowerName:        strings.ToLower(opts.Name),
		Description:      opts.Description,
		DefaultBranch:    opts.DefaultBranch,
		IsPrivate:        opts.Private,
		IsEmpty:          !opts.GitContent || templateRepo.IsEmpty,
		IsFsckEnabled:    templateRepo.IsFsckEnabled,
		TemplateID:       templateRepo.ID,
		TrustModel:       templateRepo.TrustModel,
		ObjectFormatName: templateRepo.ObjectFormatName,
		Status:           repo_model.RepositoryBeingMigrated,
	}

	// 1 - Create the repository in the database
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		return createRepositoryInDB(ctx, doer, owner, generateRepo, false)
	}); err != nil {
		return nil, err
	}

	// last - clean up the repository if something goes wrong
	defer func() {
		if err != nil {
			// we can not use the ctx because it maybe canceled or timeout
			cleanupRepository(doer, generateRepo.ID)
		}
	}()

	// 2 - check whether the repository with the same storage exists
	isExist, err := gitrepo.IsRepositoryExist(ctx, generateRepo)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", generateRepo.FullName(), err)
		return nil, err
	}
	if isExist {
		// Don't return directly, we need err in defer to cleanupRepository
		err = repo_model.ErrRepoFilesAlreadyExist{
			Uname: generateRepo.OwnerName,
			Name:  generateRepo.Name,
		}
		return nil, err
	}

	// 3 -Init git bare new repository.
	if err = git.InitRepository(ctx, generateRepo.RepoPath(), true, generateRepo.ObjectFormatName); err != nil {
		return nil, fmt.Errorf("git.InitRepository: %w", err)
	} else if err = gitrepo.CreateDelegateHooks(ctx, generateRepo); err != nil {
		return nil, fmt.Errorf("createDelegateHooks: %w", err)
	}

	// 4 - Update the git repository
	if err = updateGitRepoAfterCreate(ctx, generateRepo); err != nil {
		return nil, fmt.Errorf("updateGitRepoAfterCreate: %w", err)
	}

	// 5 - generate the repository contents according to the template
	// Git Content
	if opts.GitContent && !templateRepo.IsEmpty {
		if err = GenerateGitContent(ctx, templateRepo, generateRepo); err != nil {
			return nil, err
		}
	}

	// Topics
	if opts.Topics {
		if err = repo_model.GenerateTopics(ctx, templateRepo, generateRepo); err != nil {
			return nil, err
		}
	}

	// Git Hooks
	if opts.GitHooks {
		if err = GenerateGitHooks(ctx, templateRepo, generateRepo); err != nil {
			return nil, err
		}
	}

	// Webhooks
	if opts.Webhooks {
		if err = GenerateWebhooks(ctx, templateRepo, generateRepo); err != nil {
			return nil, err
		}
	}

	// Avatar
	if opts.Avatar && len(templateRepo.Avatar) > 0 {
		if err = generateAvatar(ctx, templateRepo, generateRepo); err != nil {
			return nil, err
		}
	}

	// Issue Labels
	if opts.IssueLabels {
		if err = GenerateIssueLabels(ctx, templateRepo, generateRepo); err != nil {
			return nil, err
		}
	}

	if opts.ProtectedBranch {
		if err = GenerateProtectedBranch(ctx, templateRepo, generateRepo); err != nil {
			return nil, err
		}
	}

	// 6 - update repository status to be ready
	generateRepo.Status = repo_model.RepositoryReady
	if err = repo_model.UpdateRepositoryCols(ctx, generateRepo, "status"); err != nil {
		return nil, fmt.Errorf("UpdateRepositoryCols: %w", err)
	}

	notify_service.CreateRepository(ctx, doer, owner, generateRepo)

	return generateRepo, nil
}
