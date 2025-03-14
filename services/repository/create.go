// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates/vars"
	"code.gitea.io/gitea/modules/util"
)

// CreateRepoOptions contains the create repository options
type CreateRepoOptions struct {
	Name             string
	Description      string
	OriginalURL      string
	GitServiceType   api.GitServiceType
	Gitignores       string
	IssueLabels      string
	License          string
	Readme           string
	DefaultBranch    string
	IsPrivate        bool
	IsMirror         bool
	IsTemplate       bool
	AutoInit         bool
	Status           repo_model.RepositoryStatus
	TrustModel       repo_model.TrustModelType
	MirrorInterval   string
	ObjectFormatName string
}

func prepareRepoCommit(ctx context.Context, repo *repo_model.Repository, tmpDir string, opts CreateRepoOptions) error {
	commitTimeStr := time.Now().Format(time.RFC3339)
	authorSig := repo.Owner.NewGitSig()

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+authorSig.Name,
		"GIT_COMMITTER_EMAIL="+authorSig.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	// Clone to temporary path and do the init commit.
	if stdout, _, err := git.NewCommand("clone").AddDynamicArguments(repo.RepoPath(), tmpDir).
		RunStdString(ctx, &git.RunOpts{Dir: "", Env: env}); err != nil {
		log.Error("Failed to clone from %v into %s: stdout: %s\nError: %v", repo, tmpDir, stdout, err)
		return fmt.Errorf("git clone: %w", err)
	}

	// README
	data, err := options.Readme(opts.Readme)
	if err != nil {
		return fmt.Errorf("GetRepoInitFile[%s]: %w", opts.Readme, err)
	}

	cloneLink := repo.CloneLink(ctx, nil /* no doer so do not generate user-related SSH link */)
	match := map[string]string{
		"Name":           repo.Name,
		"Description":    repo.Description,
		"CloneURL.SSH":   cloneLink.SSH,
		"CloneURL.HTTPS": cloneLink.HTTPS,
		"OwnerName":      repo.OwnerName,
	}
	res, err := vars.Expand(string(data), match)
	if err != nil {
		// here we could just log the error and continue the rendering
		log.Error("unable to expand template vars for repo README: %s, err: %v", opts.Readme, err)
	}
	if err = os.WriteFile(filepath.Join(tmpDir, "README.md"),
		[]byte(res), 0o644); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	// .gitignore
	if len(opts.Gitignores) > 0 {
		var buf bytes.Buffer
		names := strings.Split(opts.Gitignores, ",")
		for _, name := range names {
			data, err = options.Gitignore(name)
			if err != nil {
				return fmt.Errorf("GetRepoInitFile[%s]: %w", name, err)
			}
			buf.WriteString("# ---> " + name + "\n")
			buf.Write(data)
			buf.WriteString("\n")
		}

		if buf.Len() > 0 {
			if err = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), buf.Bytes(), 0o644); err != nil {
				return fmt.Errorf("write .gitignore: %w", err)
			}
		}
	}

	// LICENSE
	if len(opts.License) > 0 {
		data, err = repo_module.GetLicense(opts.License, &repo_module.LicenseValues{
			Owner: repo.OwnerName,
			Email: authorSig.Email,
			Repo:  repo.Name,
			Year:  time.Now().Format("2006"),
		})
		if err != nil {
			return fmt.Errorf("getLicense[%s]: %w", opts.License, err)
		}

		if err = os.WriteFile(filepath.Join(tmpDir, "LICENSE"), data, 0o644); err != nil {
			return fmt.Errorf("write LICENSE: %w", err)
		}
	}

	return nil
}

// InitRepository initializes README and .gitignore if needed.
func initRepository(ctx context.Context, u *user_model.User, repo *repo_model.Repository, opts CreateRepoOptions) (err error) {
	if err = repo_module.CheckInitRepository(ctx, repo); err != nil {
		return err
	}

	// Initialize repository according to user's choice.
	if opts.AutoInit {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "gitea-"+repo.Name)
		if err != nil {
			return fmt.Errorf("Failed to create temp dir for repository %s: %w", repo.FullName(), err)
		}
		defer func() {
			if err := util.RemoveAll(tmpDir); err != nil {
				log.Warn("Unable to remove temporary directory: %s: Error: %v", tmpDir, err)
			}
		}()

		if err = prepareRepoCommit(ctx, repo, tmpDir, opts); err != nil {
			return fmt.Errorf("prepareRepoCommit: %w", err)
		}

		// Apply changes and commit.
		if err = initRepoCommit(ctx, tmpDir, repo, u, opts.DefaultBranch); err != nil {
			return fmt.Errorf("initRepoCommit: %w", err)
		}
	}

	// Re-fetch the repository from database before updating it (else it would
	// override changes that were done earlier with sql)
	if repo, err = repo_model.GetRepositoryByID(ctx, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %w", err)
	}

	if !opts.AutoInit {
		repo.IsEmpty = true
	}

	repo.DefaultBranch = setting.Repository.DefaultBranch
	repo.DefaultWikiBranch = setting.Repository.DefaultBranch

	if len(opts.DefaultBranch) > 0 {
		repo.DefaultBranch = opts.DefaultBranch
		if err = gitrepo.SetDefaultBranch(ctx, repo, repo.DefaultBranch); err != nil {
			return fmt.Errorf("setDefaultBranch: %w", err)
		}

		if !repo.IsEmpty {
			if _, err := repo_module.SyncRepoBranches(ctx, repo.ID, u.ID); err != nil {
				return fmt.Errorf("SyncRepoBranches: %w", err)
			}
		}
	}

	if err = UpdateRepository(ctx, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	return nil
}

// CreateRepositoryDirectly creates a repository for the user/organization.
func CreateRepositoryDirectly(ctx context.Context, doer, u *user_model.User, opts CreateRepoOptions) (*repo_model.Repository, error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, repo_model.ErrReachLimitOfRepo{
			Limit: u.MaxRepoCreation,
		}
	}

	if len(opts.DefaultBranch) == 0 {
		opts.DefaultBranch = setting.Repository.DefaultBranch
	}

	// Check if label template exist
	if len(opts.IssueLabels) > 0 {
		if _, err := repo_module.LoadTemplateLabelsByDisplayName(opts.IssueLabels); err != nil {
			return nil, err
		}
	}

	if opts.ObjectFormatName == "" {
		opts.ObjectFormatName = git.Sha1ObjectFormat.Name()
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
		IsTemplate:                      opts.IsTemplate,
		CloseIssuesViaCommitInAnyBranch: setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch,
		Status:                          opts.Status,
		IsEmpty:                         !opts.AutoInit,
		TrustModel:                      opts.TrustModel,
		IsMirror:                        opts.IsMirror,
		DefaultBranch:                   opts.DefaultBranch,
		DefaultWikiBranch:               setting.Repository.DefaultBranch,
		ObjectFormatName:                opts.ObjectFormatName,
	}

	var rollbackRepo *repo_model.Repository

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := CreateRepositoryByExample(ctx, doer, u, repo, false, false); err != nil {
			return err
		}

		// No need for init mirror.
		if opts.IsMirror {
			return nil
		}

		isExist, err := gitrepo.IsRepositoryExist(ctx, repo)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", repo.FullName(), err)
			return err
		}
		if isExist {
			// repo already exists - We have two or three options.
			// 1. We fail stating that the directory exists
			// 2. We create the db repository to go with this data and adopt the git repo
			// 3. We delete it and start afresh
			//
			// Previously Gitea would just delete and start afresh - this was naughty.
			// So we will now fail and delegate to other functionality to adopt or delete
			log.Error("Files already exist in %s and we are not going to adopt or delete.", repo.FullName())
			return repo_model.ErrRepoFilesAlreadyExist{
				Uname: u.Name,
				Name:  repo.Name,
			}
		}

		if err = initRepository(ctx, doer, repo, opts); err != nil {
			if err2 := util.RemoveAll(repo.RepoPath()); err2 != nil {
				log.Error("initRepository: %v", err)
				return fmt.Errorf(
					"delete repo directory %s/%s failed(2): %v", u.Name, repo.Name, err2)
			}
			return fmt.Errorf("initRepository: %w", err)
		}

		// Initialize Issue Labels if selected
		if len(opts.IssueLabels) > 0 {
			if err = repo_module.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
				rollbackRepo = repo
				rollbackRepo.OwnerID = u.ID
				return fmt.Errorf("InitializeLabels: %w", err)
			}
		}

		if err := repo_module.CheckDaemonExportOK(ctx, repo); err != nil {
			return fmt.Errorf("checkDaemonExportOK: %w", err)
		}

		if stdout, _, err := git.NewCommand("update-server-info").
			RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()}); err != nil {
			log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			rollbackRepo = repo
			rollbackRepo.OwnerID = u.ID
			return fmt.Errorf("CreateRepository(git update-server-info): %w", err)
		}

		// update licenses
		var licenses []string
		if len(opts.License) > 0 {
			licenses = append(licenses, opts.License)

			stdout, _, err := git.NewCommand("rev-parse", "HEAD").RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})
			if err != nil {
				log.Error("CreateRepository(git rev-parse HEAD) in %v: Stdout: %s\nError: %v", repo, stdout, err)
				rollbackRepo = repo
				rollbackRepo.OwnerID = u.ID
				return fmt.Errorf("CreateRepository(git rev-parse HEAD): %w", err)
			}
			if err := repo_model.UpdateRepoLicenses(ctx, repo, stdout, licenses); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if rollbackRepo != nil {
			if errDelete := DeleteRepositoryDirectly(ctx, doer, rollbackRepo.ID); errDelete != nil {
				log.Error("Rollback deleteRepository: %v", errDelete)
			}
		}

		return nil, err
	}

	return repo, nil
}

// CreateRepositoryByExample creates a repository for the user/organization.
func CreateRepositoryByExample(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository, overwriteOrAdopt, isFork bool) (err error) {
	if err = repo_model.IsUsableRepoName(repo.Name); err != nil {
		return err
	}

	has, err := repo_model.IsRepositoryModelExist(ctx, u, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %w", err)
	} else if has {
		return repo_model.ErrRepoAlreadyExist{
			Uname: u.Name,
			Name:  repo.Name,
		}
	}

	isExist, err := gitrepo.IsRepositoryExist(ctx, repo)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repo.FullName(), err)
		return err
	}
	if !overwriteOrAdopt && isExist {
		log.Error("Files already exist in %s and we are not going to adopt or delete.", repo.FullName())
		return repo_model.ErrRepoFilesAlreadyExist{
			Uname: u.Name,
			Name:  repo.Name,
		}
	}

	if err = db.Insert(ctx, repo); err != nil {
		return err
	}
	if err = repo_model.DeleteRedirect(ctx, u.ID, repo.Name); err != nil {
		return err
	}

	// insert units for repo
	defaultUnits := unit.DefaultRepoUnits
	switch {
	case isFork:
		defaultUnits = unit.DefaultForkRepoUnits
	case repo.IsMirror:
		defaultUnits = unit.DefaultMirrorRepoUnits
	case repo.IsTemplate:
		defaultUnits = unit.DefaultTemplateRepoUnits
	}
	units := make([]repo_model.RepoUnit, 0, len(defaultUnits))
	for _, tp := range defaultUnits {
		if tp == unit.TypeIssues {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &repo_model.IssuesConfig{
					EnableTimetracker:                setting.Service.DefaultEnableTimetracking,
					AllowOnlyContributorsToTrackTime: setting.Service.DefaultAllowOnlyContributorsToTrackTime,
					EnableDependencies:               setting.Service.DefaultEnableDependencies,
				},
			})
		} else if tp == unit.TypePullRequests {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &repo_model.PullRequestsConfig{
					AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true, AllowFastForwardOnly: true,
					DefaultMergeStyle: repo_model.MergeStyle(setting.Repository.PullRequest.DefaultMergeStyle),
					AllowRebaseUpdate: true,
				},
			})
		} else if tp == unit.TypeProjects {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &repo_model.ProjectsConfig{ProjectsMode: repo_model.ProjectsModeAll},
			})
		} else {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
			})
		}
	}

	if err = db.Insert(ctx, units); err != nil {
		return err
	}

	// Remember visibility preference.
	u.LastRepoVisibility = repo.IsPrivate
	if err = user_model.UpdateUserCols(ctx, u, "last_repo_visibility"); err != nil {
		return fmt.Errorf("UpdateUserCols: %w", err)
	}

	if err = user_model.IncrUserRepoNum(ctx, u.ID); err != nil {
		return fmt.Errorf("IncrUserRepoNum: %w", err)
	}
	u.NumRepos++

	// Give access to all members in teams with access to all repositories.
	if u.IsOrganization() {
		teams, err := organization.FindOrgTeams(ctx, u.ID)
		if err != nil {
			return fmt.Errorf("FindOrgTeams: %w", err)
		}
		for _, t := range teams {
			if t.IncludesAllRepositories {
				if err := addRepositoryToTeam(ctx, t, repo); err != nil {
					return fmt.Errorf("AddRepository: %w", err)
				}
			}
		}

		if isAdmin, err := access_model.IsUserRepoAdmin(ctx, repo, doer); err != nil {
			return fmt.Errorf("IsUserRepoAdmin: %w", err)
		} else if !isAdmin {
			// Make creator repo admin if it wasn't assigned automatically
			if err = AddOrUpdateCollaborator(ctx, repo, doer, perm.AccessModeAdmin); err != nil {
				return fmt.Errorf("AddCollaborator: %w", err)
			}
		}
	} else if err = access_model.RecalculateAccesses(ctx, repo); err != nil {
		// Organization automatically called this in AddRepository method.
		return fmt.Errorf("RecalculateAccesses: %w", err)
	}

	if setting.Service.AutoWatchNewRepos {
		if err = repo_model.WatchRepo(ctx, doer, repo, true); err != nil {
			return fmt.Errorf("WatchRepo: %w", err)
		}
	}

	if err = webhook.CopyDefaultWebhooksToRepo(ctx, repo.ID); err != nil {
		return fmt.Errorf("CopyDefaultWebhooksToRepo: %w", err)
	}

	return nil
}
