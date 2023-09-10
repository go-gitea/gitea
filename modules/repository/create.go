// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

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

	repoPath := repo_model.RepoPath(u.Name, repo.Name)
	isExist, err := util.IsExist(repoPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
		return err
	}
	if !overwriteOrAdopt && isExist {
		log.Error("Files already exist in %s and we are not going to adopt or delete.", repoPath)
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
	if isFork {
		defaultUnits = unit.DefaultForkRepoUnits
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
				Config: &repo_model.PullRequestsConfig{AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true, DefaultMergeStyle: repo_model.MergeStyle(setting.Repository.PullRequest.DefaultMergeStyle), AllowRebaseUpdate: true},
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
				if err := models.AddRepository(ctx, t, repo); err != nil {
					return fmt.Errorf("AddRepository: %w", err)
				}
			}
		}

		if isAdmin, err := access_model.IsUserRepoAdmin(ctx, repo, doer); err != nil {
			return fmt.Errorf("IsUserRepoAdmin: %w", err)
		} else if !isAdmin {
			// Make creator repo admin if it wasn't assigned automatically
			if err = AddCollaborator(ctx, repo, doer); err != nil {
				return fmt.Errorf("AddCollaborator: %w", err)
			}
			if err = repo_model.ChangeCollaborationAccessMode(ctx, repo, doer.ID, perm.AccessModeAdmin); err != nil {
				return fmt.Errorf("ChangeCollaborationAccessModeCtx: %w", err)
			}
		}
	} else if err = access_model.RecalculateAccesses(ctx, repo); err != nil {
		// Organization automatically called this in AddRepository method.
		return fmt.Errorf("RecalculateAccesses: %w", err)
	}

	if setting.Service.AutoWatchNewRepos {
		if err = repo_model.WatchRepo(ctx, doer.ID, repo.ID, true); err != nil {
			return fmt.Errorf("WatchRepo: %w", err)
		}
	}

	if err = webhook.CopyDefaultWebhooksToRepo(ctx, repo.ID); err != nil {
		return fmt.Errorf("CopyDefaultWebhooksToRepo: %w", err)
	}

	return nil
}

const notRegularFileMode = os.ModeSymlink | os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular

// getDirectorySize returns the disk consumption for a given path
func getDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, info os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) { // ignore the error because the file maybe deleted during traversing.
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := info.Info()
		if err != nil {
			return err
		}
		if (f.Mode() & notRegularFileMode) == 0 {
			size += f.Size()
		}
		return err
	})
	return size, err
}

// UpdateRepoSize updates the repository size, calculating it using getDirectorySize
func UpdateRepoSize(ctx context.Context, repo *repo_model.Repository) error {
	size, err := getDirectorySize(repo.RepoPath())
	if err != nil {
		return fmt.Errorf("updateSize: %w", err)
	}

	lfsSize, err := git_model.GetRepoLFSSize(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("updateSize: GetLFSMetaObjects: %w", err)
	}

	return repo_model.UpdateRepoSize(ctx, repo.ID, size, lfsSize)
}

// CheckDaemonExportOK creates/removes git-daemon-export-ok for git-daemon...
func CheckDaemonExportOK(ctx context.Context, repo *repo_model.Repository) error {
	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}

	// Create/Remove git-daemon-export-ok for git-daemon...
	daemonExportFile := path.Join(repo.RepoPath(), `git-daemon-export-ok`)

	isExist, err := util.IsExist(daemonExportFile)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", daemonExportFile, err)
		return err
	}

	isPublic := !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePublic
	if !isPublic && isExist {
		if err = util.Remove(daemonExportFile); err != nil {
			log.Error("Failed to remove %s: %v", daemonExportFile, err)
		}
	} else if isPublic && !isExist {
		if f, err := os.Create(daemonExportFile); err != nil {
			log.Error("Failed to create %s: %v", daemonExportFile, err)
		} else {
			f.Close()
		}
	}

	return nil
}

// UpdateRepository updates a repository with db context
func UpdateRepository(ctx context.Context, repo *repo_model.Repository, visibilityChanged bool) (err error) {
	repo.LowerName = strings.ToLower(repo.Name)

	e := db.GetEngine(ctx)

	if _, err = e.ID(repo.ID).AllCols().Update(repo); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	if err = UpdateRepoSize(ctx, repo); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	if visibilityChanged {
		if err = repo.LoadOwner(ctx); err != nil {
			return fmt.Errorf("LoadOwner: %w", err)
		}
		if repo.Owner.IsOrganization() {
			// Organization repository need to recalculate access table when visibility is changed.
			if err = access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %w", err)
			}
		}

		// If repo has become private, we need to set its actions to private.
		if repo.IsPrivate {
			_, err = e.Where("repo_id = ?", repo.ID).Cols("is_private").Update(&activities_model.Action{
				IsPrivate: true,
			})
			if err != nil {
				return err
			}

			if err = repo_model.ClearRepoStars(ctx, repo.ID); err != nil {
				return err
			}
		}

		// Create/Remove git-daemon-export-ok for git-daemon...
		if err := CheckDaemonExportOK(ctx, repo); err != nil {
			return err
		}

		forkRepos, err := repo_model.GetRepositoriesByForkID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("getRepositoriesByForkID: %w", err)
		}
		for i := range forkRepos {
			forkRepos[i].IsPrivate = repo.IsPrivate || repo.Owner.Visibility == api.VisibleTypePrivate
			if err = UpdateRepository(ctx, forkRepos[i], true); err != nil {
				return fmt.Errorf("updateRepository[%d]: %w", forkRepos[i].ID, err)
			}
		}

		// If visibility is changed, we need to update the issue indexer.
		// Since the data in the issue indexer have field to indicate if the repo is public or not.
		issue_indexer.UpdateRepoIndexer(ctx, repo.ID)
	}

	return nil
}
