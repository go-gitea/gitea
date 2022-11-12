// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
)

// AdoptRepository adopts pre-existing repository files for the user/organization.
func AdoptRepository(doer, u *user_model.User, opts repo_module.CreateRepoOptions) (*repo_model.Repository, error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, repo_model.ErrReachLimitOfRepo{
			Limit: u.MaxRepoCreation,
		}
	}

	if len(opts.DefaultBranch) == 0 {
		opts.DefaultBranch = setting.Repository.DefaultBranch
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

	if err := db.WithTx(func(ctx context.Context) error {
		repoPath := repo_model.RepoPath(u.Name, repo.Name)
		isExist, err := util.IsExist(repoPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
			return err
		}
		if !isExist {
			return repo_model.ErrRepoNotExist{
				OwnerName: u.Name,
				Name:      repo.Name,
			}
		}

		if err := repo_module.CreateRepositoryByExample(ctx, doer, u, repo, true); err != nil {
			return err
		}
		if err := adoptRepository(ctx, repoPath, doer, repo, opts); err != nil {
			return fmt.Errorf("createDelegateHooks: %w", err)
		}
		if err := repo_module.CheckDaemonExportOK(ctx, repo); err != nil {
			return fmt.Errorf("checkDaemonExportOK: %w", err)
		}

		// Initialize Issue Labels if selected
		if len(opts.IssueLabels) > 0 {
			if err := repo_module.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
				return fmt.Errorf("InitializeLabels: %w", err)
			}
		}

		if stdout, _, err := git.NewCommand(ctx, "update-server-info").
			SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
			RunStdString(&git.RunOpts{Dir: repoPath}); err != nil {
			log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			return fmt.Errorf("CreateRepository(git update-server-info): %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	notification.NotifyCreateRepository(doer, u, repo)

	return repo, nil
}

func adoptRepository(ctx context.Context, repoPath string, u *user_model.User, repo *repo_model.Repository, opts repo_module.CreateRepoOptions) (err error) {
	isExist, err := util.IsExist(repoPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
		return err
	}
	if !isExist {
		return fmt.Errorf("adoptRepository: path does not already exist: %s", repoPath)
	}

	if err := repo_module.CreateDelegateHooks(repoPath); err != nil {
		return fmt.Errorf("createDelegateHooks: %w", err)
	}

	// Re-fetch the repository from database before updating it (else it would
	// override changes that were done earlier with sql)
	if repo, err = repo_model.GetRepositoryByIDCtx(ctx, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %w", err)
	}

	repo.IsEmpty = false

	// Don't bother looking this repo in the context it won't be there
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		return fmt.Errorf("openRepository: %w", err)
	}
	defer gitRepo.Close()

	if len(opts.DefaultBranch) > 0 {
		repo.DefaultBranch = opts.DefaultBranch

		if err = gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
			return fmt.Errorf("setDefaultBranch: %w", err)
		}
	} else {
		repo.DefaultBranch, err = gitRepo.GetDefaultBranch()
		if err != nil {
			repo.DefaultBranch = setting.Repository.DefaultBranch
			if err = gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
				return fmt.Errorf("setDefaultBranch: %w", err)
			}
		}
	}
	branches, _, _ := gitRepo.GetBranchNames(0, 0)
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

		if err = gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
			return fmt.Errorf("setDefaultBranch: %w", err)
		}
	}

	if err = repo_module.UpdateRepository(ctx, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	return nil
}

// DeleteUnadoptedRepository deletes unadopted repository files from the filesystem
func DeleteUnadoptedRepository(doer, u *user_model.User, repoName string) error {
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

	if exist, err := repo_model.IsRepositoryExist(db.DefaultContext, u, repoName); err != nil {
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

func checkUnadoptedRepositories(userName string, repoNamesToCheck []string, unadopted *unadoptedRepositories) error {
	if len(repoNamesToCheck) == 0 {
		return nil
	}
	ctxUser, err := user_model.GetUserByName(db.DefaultContext, userName)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			log.Debug("Missing user: %s", userName)
			return nil
		}
		return err
	}
	repos, _, err := repo_model.GetUserRepositories(&repo_model.SearchRepoOptions{
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
func ListUnadoptedRepositories(query string, opts *db.ListOptions) ([]string, int, error) {
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
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() || path == root {
			return nil
		}

		if !strings.ContainsRune(path[len(root)+1:], filepath.Separator) {
			// Got a new user
			if err = checkUnadoptedRepositories(userName, repoNamesToCheck, unadopted); err != nil {
				return err
			}
			repoNamesToCheck = repoNamesToCheck[:0]

			if !globUser.Match(info.Name()) {
				return filepath.SkipDir
			}

			userName = info.Name()
			return nil
		}

		name := info.Name()

		if !strings.HasSuffix(name, ".git") {
			return filepath.SkipDir
		}
		name = name[:len(name)-4]
		if repo_model.IsUsableRepoName(name) != nil || strings.ToLower(name) != name || !globRepo.Match(name) {
			return filepath.SkipDir
		}

		repoNamesToCheck = append(repoNamesToCheck, name)
		if len(repoNamesToCheck) > setting.Database.IterateBufferSize {
			if err = checkUnadoptedRepositories(userName, repoNamesToCheck, unadopted); err != nil {
				return err
			}
			repoNamesToCheck = repoNamesToCheck[:0]

		}
		return filepath.SkipDir
	}); err != nil {
		return nil, 0, err
	}

	if err := checkUnadoptedRepositories(userName, repoNamesToCheck, unadopted); err != nil {
		return nil, 0, err
	}

	return unadopted.repositories, unadopted.index, nil
}
