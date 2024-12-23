// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/setting"
)

// TeamAddRepository adds new repository to team of organization.
func TeamAddRepository(ctx context.Context, t *organization.Team, repo *repo_model.Repository) (err error) {
	if repo.OwnerID != t.OrgID {
		return errors.New("repository does not belong to organization")
	} else if organization.HasTeamRepo(ctx, t.OrgID, t.ID, repo.ID) {
		return nil
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		return addRepositoryToTeam(ctx, t, repo)
	})
}

func addRepositoryToTeam(ctx context.Context, t *organization.Team, repo *repo_model.Repository) (err error) {
	if err = organization.AddTeamRepo(ctx, t.OrgID, t.ID, repo.ID); err != nil {
		return err
	}

	if err = organization.IncrTeamRepoNum(ctx, t.ID); err != nil {
		return fmt.Errorf("update team: %w", err)
	}

	t.NumRepos++

	if err = access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
		return fmt.Errorf("recalculateAccesses: %w", err)
	}

	// Make all team members watch this repo if enabled in global settings
	if setting.Service.AutoWatchNewRepos {
		if err = t.LoadMembers(ctx); err != nil {
			return fmt.Errorf("getMembers: %w", err)
		}
		for _, u := range t.Members {
			if err = repo_model.WatchRepo(ctx, u, repo, true); err != nil {
				return fmt.Errorf("watchRepo: %w", err)
			}
		}
	}

	return nil
}

// AddAllRepositoriesToTeam adds all repositories to the team.
// If the team already has some repositories they will be left unchanged.
func AddAllRepositoriesToTeam(ctx context.Context, t *organization.Team) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		orgRepos, err := repo_model.GetOrgRepositories(ctx, t.OrgID)
		if err != nil {
			return fmt.Errorf("get org repos: %w", err)
		}

		for _, repo := range orgRepos {
			if !organization.HasTeamRepo(ctx, t.OrgID, t.ID, repo.ID) {
				if err := addRepositoryToTeam(ctx, t, repo); err != nil {
					return fmt.Errorf("AddRepository: %w", err)
				}
			}
		}

		return nil
	})
}

// RemoveAllRepositoriesFromTeam removes all repositories from team and recalculates access
func RemoveAllRepositoriesFromTeam(ctx context.Context, t *organization.Team) (err error) {
	if t.IncludesAllRepositories {
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = removeAllRepositoriesFromTeam(ctx, t); err != nil {
		return err
	}

	return committer.Commit()
}

// removeAllRepositoriesFromTeam removes all repositories from team and recalculates access
// Note: Shall not be called if team includes all repositories
func removeAllRepositoriesFromTeam(ctx context.Context, t *organization.Team) (err error) {
	e := db.GetEngine(ctx)
	repos, err := repo_model.GetTeamRepositories(ctx, &repo_model.SearchTeamRepoOptions{
		TeamID: t.ID,
	})
	if err != nil {
		return fmt.Errorf("GetTeamRepositories: %w", err)
	}

	// Delete all accesses.
	for _, repo := range repos {
		if err := access_model.RecalculateTeamAccesses(ctx, repo, t.ID); err != nil {
			return err
		}

		// Remove watches from all users and now unaccessible repos
		for _, user := range t.Members {
			has, err := access_model.HasAnyUnitAccess(ctx, user.ID, repo)
			if err != nil {
				return err
			} else if has {
				continue
			}

			if err = repo_model.WatchRepo(ctx, user, repo, false); err != nil {
				return err
			}

			// Remove all IssueWatches a user has subscribed to in the repositories
			if err = issues_model.RemoveIssueWatchersByRepoID(ctx, user.ID, repo.ID); err != nil {
				return err
			}
		}
	}

	// Delete team-repo
	if _, err := e.
		Where("team_id=?", t.ID).
		Delete(new(organization.TeamRepo)); err != nil {
		return err
	}

	t.NumRepos = 0
	if _, err = e.ID(t.ID).Cols("num_repos").Update(t); err != nil {
		return err
	}

	return nil
}

// RemoveRepositoryFromTeam removes repository from team of organization.
// If the team shall include all repositories the request is ignored.
func RemoveRepositoryFromTeam(ctx context.Context, t *organization.Team, repoID int64) error {
	if !HasRepository(ctx, t, repoID) {
		return nil
	}

	if t.IncludesAllRepositories {
		return nil
	}

	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = removeRepositoryFromTeam(ctx, t, repo, true); err != nil {
		return err
	}

	return committer.Commit()
}

// removeRepositoryFromTeam removes a repository from a team and recalculates access
// Note: Repository shall not be removed from team if it includes all repositories (unless the repository is deleted)
func removeRepositoryFromTeam(ctx context.Context, t *organization.Team, repo *repo_model.Repository, recalculate bool) (err error) {
	e := db.GetEngine(ctx)
	if err = organization.RemoveTeamRepo(ctx, t.ID, repo.ID); err != nil {
		return err
	}

	t.NumRepos--
	if _, err = e.ID(t.ID).Cols("num_repos").Update(t); err != nil {
		return err
	}

	// Don't need to recalculate when delete a repository from organization.
	if recalculate {
		if err = access_model.RecalculateTeamAccesses(ctx, repo, t.ID); err != nil {
			return err
		}
	}

	teamMembers, err := organization.GetTeamMembers(ctx, &organization.SearchMembersOptions{
		TeamID: t.ID,
	})
	if err != nil {
		return fmt.Errorf("GetTeamMembers: %w", err)
	}
	for _, member := range teamMembers {
		has, err := access_model.HasAnyUnitAccess(ctx, member.ID, repo)
		if err != nil {
			return err
		} else if has {
			continue
		}

		if err = repo_model.WatchRepo(ctx, member, repo, false); err != nil {
			return err
		}

		// Remove all IssueWatches a user has subscribed to in the repositories
		if err := issues_model.RemoveIssueWatchersByRepoID(ctx, member.ID, repo.ID); err != nil {
			return err
		}
	}

	return nil
}

// HasRepository returns true if given repository belong to team.
func HasRepository(ctx context.Context, t *organization.Team, repoID int64) bool {
	return organization.HasTeamRepo(ctx, t.OrgID, t.ID, repoID)
}
