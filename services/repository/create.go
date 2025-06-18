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
	system_model "code.gitea.io/gitea/models/system"
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
		names := strings.SplitSeq(opts.Gitignores, ",")
		for name := range names {
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
	// Init git bare new repository.
	if err = git.InitRepository(ctx, repo.RepoPath(), true, repo.ObjectFormatName); err != nil {
		return fmt.Errorf("git.InitRepository: %w", err)
	} else if err = gitrepo.CreateDelegateHooks(ctx, repo); err != nil {
		return fmt.Errorf("createDelegateHooks: %w", err)
	}

	// Initialize repository according to user's choice.
	if opts.AutoInit {
		tmpDir, cleanup, err := setting.AppDataTempDir("git-repo-content").MkdirTempRandom("repos-" + repo.Name)
		if err != nil {
			return fmt.Errorf("failed to create temp dir for repository %s: %w", repo.FullName(), err)
		}
		defer cleanup()

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

	if err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_empty", "default_branch", "default_wiki_branch"); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	if err = repo_module.UpdateRepoSize(ctx, repo); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	return nil
}

// CreateRepositoryDirectly creates a repository for the user/organization.
// if needsUpdateToReady is true, it will update the repository status to ready when success
func CreateRepositoryDirectly(ctx context.Context, doer, owner *user_model.User,
	opts CreateRepoOptions, needsUpdateToReady bool,
) (*repo_model.Repository, error) {
	if !doer.CanCreateRepoIn(owner) {
		return nil, repo_model.ErrReachLimitOfRepo{
			Limit: owner.MaxRepoCreation,
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
			cleanupRepository(repo.ID)
		}
	}()

	// No need for init mirror.
	if opts.IsMirror {
		return repo, nil
	}

	// 2 - check whether the repository with the same storage exists
	var isExist bool
	isExist, err = gitrepo.IsRepositoryExist(ctx, repo)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repo.FullName(), err)
		return nil, err
	}
	if isExist {
		log.Error("Files already exist in %s and we are not going to adopt or delete.", repo.FullName())
		// Don't return directly, we need err in defer to cleanupRepository
		err = repo_model.ErrRepoFilesAlreadyExist{
			Uname: repo.OwnerName,
			Name:  repo.Name,
		}
		return nil, err
	}

	// 3 - init git repository in storage
	if err = initRepository(ctx, doer, repo, opts); err != nil {
		return nil, fmt.Errorf("initRepository: %w", err)
	}

	// 4 - Initialize Issue Labels if selected
	if len(opts.IssueLabels) > 0 {
		if err = repo_module.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
			return nil, fmt.Errorf("InitializeLabels: %w", err)
		}
	}

	// 5 - Update the git repository
	if err = updateGitRepoAfterCreate(ctx, repo); err != nil {
		return nil, fmt.Errorf("updateGitRepoAfterCreate: %w", err)
	}

	// 6 - update licenses
	var licenses []string
	if len(opts.License) > 0 {
		licenses = append(licenses, opts.License)

		var stdout string
		stdout, _, err = git.NewCommand("rev-parse", "HEAD").RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})
		if err != nil {
			log.Error("CreateRepository(git rev-parse HEAD) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			return nil, fmt.Errorf("CreateRepository(git rev-parse HEAD): %w", err)
		}
		if err = repo_model.UpdateRepoLicenses(ctx, repo, stdout, licenses); err != nil {
			return nil, err
		}
	}

	// 7 - update repository status to be ready
	if needsUpdateToReady {
		repo.Status = repo_model.RepositoryReady
		if err = repo_model.UpdateRepositoryColsWithAutoTime(ctx, repo, "status"); err != nil {
			return nil, fmt.Errorf("UpdateRepositoryCols: %w", err)
		}
	}

	return repo, nil
}

// createRepositoryInDB creates a repository for the user/organization.
func createRepositoryInDB(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository, isFork bool) (err error) {
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
		switch tp {
		case unit.TypeIssues:
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &repo_model.IssuesConfig{
					EnableTimetracker:                setting.Service.DefaultEnableTimetracking,
					AllowOnlyContributorsToTrackTime: setting.Service.DefaultAllowOnlyContributorsToTrackTime,
					EnableDependencies:               setting.Service.DefaultEnableDependencies,
				},
			})
		case unit.TypePullRequests:
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &repo_model.PullRequestsConfig{
					AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true, AllowFastForwardOnly: true,
					DefaultMergeStyle: repo_model.MergeStyle(setting.Repository.PullRequest.DefaultMergeStyle),
					AllowRebaseUpdate: true,
				},
			})
		case unit.TypeProjects:
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &repo_model.ProjectsConfig{ProjectsMode: repo_model.ProjectsModeAll},
			})
		default:
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

func cleanupRepository(repoID int64) {
	if errDelete := DeleteRepositoryDirectly(db.DefaultContext, repoID); errDelete != nil {
		log.Error("cleanupRepository failed: %v", errDelete)
		// add system notice
		if err := system_model.CreateRepositoryNotice("DeleteRepositoryDirectly failed when cleanup repository: %v", errDelete); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
}

func updateGitRepoAfterCreate(ctx context.Context, repo *repo_model.Repository) error {
	if err := checkDaemonExportOK(ctx, repo); err != nil {
		return fmt.Errorf("checkDaemonExportOK: %w", err)
	}

	if stdout, _, err := git.NewCommand("update-server-info").
		RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()}); err != nil {
		log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
		return fmt.Errorf("CreateRepository(git update-server-info): %w", err)
	}
	return nil
}
