// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"

	"github.com/gobwas/glob"
)

func deleteFailedAdoptRepository(repoID int64) error {
	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		if err := deleteDBRepository(ctx, repoID); err != nil {
			return fmt.Errorf("deleteDBRepository: %w", err)
		}
		if err := git_model.DeleteRepoBranches(ctx, repoID); err != nil {
			return fmt.Errorf("deleteRepoBranches: %w", err)
		}
		return repo_model.DeleteRepoReleases(ctx, repoID)
	})
}

// AdoptRepository adopts pre-existing repository files for the user/organization.
func AdoptRepository(ctx context.Context, doer, owner *user_model.User, opts CreateRepoOptions) (*repo_model.Repository, error) {
	if !doer.CanCreateRepoIn(owner) {
		return nil, repo_model.ErrReachLimitOfRepo{
			Limit: owner.MaxRepoCreation,
		}
	}

	repo := &repo_model.Repository{
		OwnerID:                         owner.ID,
		Owner:                           owner,
		OwnerName:                       owner.Name,
		Name:                            opts.Name,
		LowerName:                       strings.ToLower(opts.Name),
		Description:                     opts.Description,
		OriginalURL:                     opts.OriginalURL,
		OriginalServiceType:             opts.GitServiceType,
		IsPrivate:                       opts.IsPrivate,
		IsFsckEnabled:                   !opts.IsMirror,
		CloseIssuesViaCommitInAnyBranch: setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch,
		Status:                          repo_model.RepositoryBeingMigrated,
		IsEmpty:                         !opts.AutoInit,
	}

	// 1 - create the repository database operations first
	err := db.WithTx(ctx, func(ctx context.Context) error {
		return createRepositoryInDB(ctx, doer, owner, repo, false)
	})
	if err != nil {
		return nil, err
	}

	// last - clean up if something goes wrong
	// WARNING: Don't override all later err with local variables
	defer func() {
		if err != nil {
			// we can not use the ctx because it maybe canceled or timeout
			if errDel := deleteFailedAdoptRepository(repo.ID); errDel != nil {
				log.Error("Failed to delete repository %s that could not be adopted: %v", repo.FullName(), errDel)
			}
		}
	}()

	// Re-fetch the repository from database before updating it (else it would
	// override changes that were done earlier with sql)
	if repo, err = repo_model.GetRepositoryByID(ctx, repo.ID); err != nil {
		return nil, fmt.Errorf("getRepositoryByID: %w", err)
	}

	// 2 - adopt the repository from disk
	if err = adoptRepository(ctx, repo, opts.DefaultBranch); err != nil {
		return nil, fmt.Errorf("adoptRepository: %w", err)
	}

	// 3 - Update the git repository
	if err = updateGitRepoAfterCreate(ctx, repo); err != nil {
		return nil, fmt.Errorf("updateGitRepoAfterCreate: %w", err)
	}

	// 4 - update repository status
	repo.Status = repo_model.RepositoryReady
	if err = repo_model.UpdateRepositoryCols(ctx, repo, "status"); err != nil {
		return nil, fmt.Errorf("UpdateRepositoryCols: %w", err)
	}

	notify_service.AdoptRepository(ctx, doer, owner, repo)

	return repo, nil
}

func adoptRepository(ctx context.Context, repo *repo_model.Repository, defaultBranch string) (err error) {
	isExist, err := gitrepo.IsRepositoryExist(ctx, repo)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repo.FullName(), err)
		return err
	}
	if !isExist {
		return fmt.Errorf("adoptRepository: path does not already exist: %s", repo.FullName())
	}

	if err := gitrepo.CreateDelegateHooks(ctx, repo); err != nil {
		return fmt.Errorf("createDelegateHooks: %w", err)
	}

	repo.IsEmpty = false

	if len(defaultBranch) > 0 {
		repo.DefaultBranch = defaultBranch

		if err = gitrepo.SetDefaultBranch(ctx, repo, repo.DefaultBranch); err != nil {
			return fmt.Errorf("setDefaultBranch: %w", err)
		}
	} else {
		repo.DefaultBranch, err = gitrepo.GetDefaultBranch(ctx, repo)
		if err != nil {
			repo.DefaultBranch = setting.Repository.DefaultBranch
			if err = gitrepo.SetDefaultBranch(ctx, repo, repo.DefaultBranch); err != nil {
				return fmt.Errorf("setDefaultBranch: %w", err)
			}
		}
	}

	// Don't bother looking this repo in the context it won't be there
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return fmt.Errorf("openRepository: %w", err)
	}
	defer gitRepo.Close()

	if _, err = repo_module.SyncRepoBranchesWithRepo(ctx, repo, gitRepo, 0); err != nil {
		return fmt.Errorf("SyncRepoBranchesWithRepo: %w", err)
	}

	if err = repo_module.SyncReleasesWithTags(ctx, repo, gitRepo); err != nil {
		return fmt.Errorf("SyncReleasesWithTags: %w", err)
	}

	branches, _ := git_model.FindBranchNames(ctx, git_model.FindBranchOptions{
		RepoID:          repo.ID,
		ListOptions:     db.ListOptionsAll,
		IsDeletedBranch: optional.Some(false),
	})

	found := false
	hasDefault := false
	hasMaster := false
	hasMain := false
	for _, branch := range branches {
		if branch == repo.DefaultBranch {
			found = true
			break
		} else if branch == setting.Repository.DefaultBranch {
			hasDefault = true
		} else if branch == "master" {
			hasMaster = true
		} else if branch == "main" {
			hasMain = true
		}
	}
	if !found {
		if hasDefault {
			repo.DefaultBranch = setting.Repository.DefaultBranch
		} else if hasMaster {
			repo.DefaultBranch = "master"
		} else if hasMain {
			repo.DefaultBranch = "main"
		} else if len(branches) > 0 {
			repo.DefaultBranch = branches[0]
		} else {
			repo.IsEmpty = true
			repo.DefaultBranch = setting.Repository.DefaultBranch
		}

		if err = gitrepo.SetDefaultBranch(ctx, repo, repo.DefaultBranch); err != nil {
			return fmt.Errorf("setDefaultBranch: %w", err)
		}
	}
	if err = updateRepository(ctx, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	return nil
}

// DeleteUnadoptedRepository deletes unadopted repository files from the filesystem
func DeleteUnadoptedRepository(ctx context.Context, doer, u *user_model.User, repoName string) error {
	if err := repo_model.IsUsableRepoName(repoName); err != nil {
		return err
	}

	repoPath := repo_model.RepoPath(u.Name, repoName)
	isExist, err := util.IsExist(repoPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
		return err
	}
	if !isExist {
		return repo_model.ErrRepoNotExist{
			OwnerName: u.Name,
			Name:      repoName,
		}
	}

	if exist, err := repo_model.IsRepositoryModelExist(ctx, u, repoName); err != nil {
		return err
	} else if exist {
		return repo_model.ErrRepoAlreadyExist{
			Uname: u.Name,
			Name:  repoName,
		}
	}

	return util.RemoveAll(repoPath)
}

type unadoptedRepositories struct {
	repositories []string
	index        int
	start        int
	end          int
}

func (unadopted *unadoptedRepositories) add(repository string) {
	if unadopted.index >= unadopted.start && unadopted.index < unadopted.end {
		unadopted.repositories = append(unadopted.repositories, repository)
	}
	unadopted.index++
}

func checkUnadoptedRepositories(ctx context.Context, userName string, repoNamesToCheck []string, unadopted *unadoptedRepositories) error {
	if len(repoNamesToCheck) == 0 {
		return nil
	}
	ctxUser, err := user_model.GetUserByName(ctx, userName)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			log.Debug("Missing user: %s", userName)
			return nil
		}
		return err
	}
	repos, _, err := repo_model.GetUserRepositories(ctx, &repo_model.SearchRepoOptions{
		Actor:   ctxUser,
		Private: true,
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: len(repoNamesToCheck),
		}, LowerNames: repoNamesToCheck,
	})
	if err != nil {
		return err
	}
	if len(repos) == len(repoNamesToCheck) {
		return nil
	}
	repoNames := make(container.Set[string], len(repos))
	for _, repo := range repos {
		repoNames.Add(repo.LowerName)
	}
	for _, repoName := range repoNamesToCheck {
		if !repoNames.Contains(repoName) {
			unadopted.add(path.Join(userName, repoName)) // These are not used as filepaths - but as reponames - therefore use path.Join not filepath.Join
		}
	}
	return nil
}

// ListUnadoptedRepositories lists all the unadopted repositories that match the provided query
func ListUnadoptedRepositories(ctx context.Context, query string, opts *db.ListOptions) ([]string, int, error) {
	globUser, _ := glob.Compile("*")
	globRepo, _ := glob.Compile("*")

	qsplit := strings.SplitN(query, "/", 2)
	if len(qsplit) > 0 && len(query) > 0 {
		var err error
		globUser, err = glob.Compile(qsplit[0])
		if err != nil {
			log.Info("Invalid glob expression '%s' (skipped): %v", qsplit[0], err)
		}
		if len(qsplit) > 1 {
			globRepo, err = glob.Compile(qsplit[1])
			if err != nil {
				log.Info("Invalid glob expression '%s' (skipped): %v", qsplit[1], err)
			}
		}
	}
	var repoNamesToCheck []string

	start := (opts.Page - 1) * opts.PageSize
	unadopted := &unadoptedRepositories{
		repositories: make([]string, 0, opts.PageSize),
		start:        start,
		end:          start + opts.PageSize,
		index:        0,
	}

	var userName string

	// We're going to iterate by pagesize.
	root := filepath.Clean(setting.RepoRootPath)
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || path == root {
			return nil
		}

		name := d.Name()

		if !strings.ContainsRune(path[len(root)+1:], filepath.Separator) {
			// Got a new user
			if err = checkUnadoptedRepositories(ctx, userName, repoNamesToCheck, unadopted); err != nil {
				return err
			}
			repoNamesToCheck = repoNamesToCheck[:0]

			if !globUser.Match(name) {
				return filepath.SkipDir
			}

			userName = name
			return nil
		}

		if !strings.HasSuffix(name, ".git") {
			return filepath.SkipDir
		}
		name = name[:len(name)-4]
		if repo_model.IsUsableRepoName(name) != nil || strings.ToLower(name) != name || !globRepo.Match(name) {
			return filepath.SkipDir
		}

		repoNamesToCheck = append(repoNamesToCheck, name)
		if len(repoNamesToCheck) >= setting.Database.IterateBufferSize {
			if err = checkUnadoptedRepositories(ctx, userName, repoNamesToCheck, unadopted); err != nil {
				return err
			}
			repoNamesToCheck = repoNamesToCheck[:0]
		}
		return filepath.SkipDir
	}); err != nil {
		return nil, 0, err
	}

	if err := checkUnadoptedRepositories(ctx, userName, repoNamesToCheck, unadopted); err != nil {
		return nil, 0, err
	}

	return unadopted.repositories, unadopted.index, nil
}
