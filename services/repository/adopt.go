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
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"

	"github.com/gobwas/glob"
)

// AdoptRepository adopts pre-existing repository files for the user/organization.
func AdoptRepository(ctx context.Context, doer, u *user_model.User, opts CreateRepoOptions) (*repo_model.Repository, error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, repo_model.ErrReachLimitOfRepo{
			Limit: u.MaxRepoCreation,
		}
	}

	repo := &repo_model.Repository{
		OwnerID:                         u.ID,
		Owner:                           u,
		OwnerName:                       u.Name,
		Name:                            opts.Name,
		LowerName:                       strings.ToLower(opts.Name),
		Description:                     opts.Description,
		OriginalURL:                     opts.OriginalURL,
		OriginalServiceType:             opts.GitServiceType,
		IsPrivate:                       opts.IsPrivate,
		IsFsckEnabled:                   !opts.IsMirror,
		CloseIssuesViaCommitInAnyBranch: setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch,
		Status:                          opts.Status,
		IsEmpty:                         !opts.AutoInit,
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		isExist, err := gitrepo.IsRepositoryExist(ctx, repo)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", repo.FullName(), err)
			return err
		}
		if !isExist {
			return repo_model.ErrRepoNotExist{
				OwnerName: u.Name,
				Name:      repo.Name,
			}
		}

		if err := CreateRepositoryByExample(ctx, doer, u, repo, true, false); err != nil {
			return err
		}

		// Re-fetch the repository from database before updating it (else it would
		// override changes that were done earlier with sql)
		if repo, err = repo_model.GetRepositoryByID(ctx, repo.ID); err != nil {
			return fmt.Errorf("getRepositoryByID: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := func() error {
		if err := adoptRepository(ctx, repo, opts.DefaultBranch); err != nil {
			return fmt.Errorf("adoptRepository: %w", err)
		}

		if err := repo_module.CheckDaemonExportOK(ctx, repo); err != nil {
			return fmt.Errorf("checkDaemonExportOK: %w", err)
		}

		if stdout, _, err := git.NewCommand("update-server-info").
			RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()}); err != nil {
			log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			return fmt.Errorf("CreateRepository(git update-server-info): %w", err)
		}
		return nil
	}(); err != nil {
		if errDel := DeleteRepository(ctx, doer, repo, false /* no notify */); errDel != nil {
			log.Error("Failed to delete repository %s that could not be adopted: %v", repo.FullName(), errDel)
		}
		return nil, err
	}
	notify_service.AdoptRepository(ctx, doer, u, repo)

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

	if err := repo_module.CreateDelegateHooks(repo.RepoPath()); err != nil {
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
	if err = repo_module.UpdateRepository(ctx, repo, false); err != nil {
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
