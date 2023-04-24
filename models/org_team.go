// Copyright 2018 The Gitea Authors. All rights reserved.
// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

func AddRepository(ctx context.Context, t *organization.Team, repo *repo_model.Repository) (err error) {
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
			if err = repo_model.WatchRepo(ctx, u.ID, repo.ID, true); err != nil {
				return fmt.Errorf("watchRepo: %w", err)
			}
		}
	}

	return nil
}

// addAllRepositories adds all repositories to the team.
// If the team already has some repositories they will be left unchanged.
func addAllRepositories(ctx context.Context, t *organization.Team) error {
	orgRepos, err := organization.GetOrgRepositories(ctx, t.OrgID)
	if err != nil {
		return fmt.Errorf("get org repos: %w", err)
	}

	for _, repo := range orgRepos {
		if !organization.HasTeamRepo(ctx, t.OrgID, t.ID, repo.ID) {
			if err := AddRepository(ctx, t, repo); err != nil {
				return fmt.Errorf("AddRepository: %w", err)
			}
		}
	}

	return nil
}

// AddAllRepositories adds all repositories to the team
func AddAllRepositories(t *organization.Team) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = addAllRepositories(ctx, t); err != nil {
		return err
	}

	return committer.Commit()
}

// RemoveAllRepositories removes all repositories from team and recalculates access
func RemoveAllRepositories(t *organization.Team) (err error) {
	if t.IncludesAllRepositories {
		return nil
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = removeAllRepositories(ctx, t); err != nil {
		return err
	}

	return committer.Commit()
}

// removeAllRepositories removes all repositories from team and recalculates access
// Note: Shall not be called if team includes all repositories
func removeAllRepositories(ctx context.Context, t *organization.Team) (err error) {
	e := db.GetEngine(ctx)
	// Delete all accesses.
	for _, repo := range t.Repos {
		if err := access_model.RecalculateTeamAccesses(ctx, repo, t.ID); err != nil {
			return err
		}

		// Remove watches from all users and now unaccessible repos
		for _, user := range t.Members {
			has, err := access_model.HasAccess(ctx, user.ID, repo)
			if err != nil {
				return err
			} else if has {
				continue
			}

			if err = repo_model.WatchRepo(ctx, user.ID, repo.ID, false); err != nil {
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

// HasRepository returns true if given repository belong to team.
func HasRepository(t *organization.Team, repoID int64) bool {
	return organization.HasTeamRepo(db.DefaultContext, t.OrgID, t.ID, repoID)
}

// removeRepository removes a repository from a team and recalculates access
// Note: Repository shall not be removed from team if it includes all repositories (unless the repository is deleted)
func removeRepository(ctx context.Context, t *organization.Team, repo *repo_model.Repository, recalculate bool) (err error) {
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

	teamUsers, err := organization.GetTeamUsersByTeamID(ctx, t.ID)
	if err != nil {
		return fmt.Errorf("getTeamUsersByTeamID: %w", err)
	}
	for _, teamUser := range teamUsers {
		has, err := access_model.HasAccess(ctx, teamUser.UID, repo)
		if err != nil {
			return err
		} else if has {
			continue
		}

		if err = repo_model.WatchRepo(ctx, teamUser.UID, repo.ID, false); err != nil {
			return err
		}

		// Remove all IssueWatches a user has subscribed to in the repositories
		if err := issues_model.RemoveIssueWatchersByRepoID(ctx, teamUser.UID, repo.ID); err != nil {
			return err
		}
	}

	return nil
}

// RemoveRepository removes repository from team of organization.
// If the team shall include all repositories the request is ignored.
func RemoveRepository(t *organization.Team, repoID int64) error {
	if !HasRepository(t, repoID) {
		return nil
	}

	if t.IncludesAllRepositories {
		return nil
	}

	repo, err := repo_model.GetRepositoryByID(db.DefaultContext, repoID)
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = removeRepository(ctx, t, repo, true); err != nil {
		return err
	}

	return committer.Commit()
}

// NewTeam creates a record of new team.
// It's caller's responsibility to assign organization ID.
func NewTeam(t *organization.Team) (err error) {
	if len(t.Name) == 0 {
		return util.NewInvalidArgumentErrorf("empty team name")
	}

	if err = organization.IsUsableTeamName(t.Name); err != nil {
		return err
	}

	has, err := db.GetEngine(db.DefaultContext).ID(t.OrgID).Get(new(user_model.User))
	if err != nil {
		return err
	}
	if !has {
		return organization.ErrOrgNotExist{ID: t.OrgID}
	}

	t.LowerName = strings.ToLower(t.Name)
	has, err = db.GetEngine(db.DefaultContext).
		Where("org_id=?", t.OrgID).
		And("lower_name=?", t.LowerName).
		Get(new(organization.Team))
	if err != nil {
		return err
	}
	if has {
		return organization.ErrTeamAlreadyExist{OrgID: t.OrgID, Name: t.LowerName}
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = db.Insert(ctx, t); err != nil {
		return err
	}

	// insert units for team
	if len(t.Units) > 0 {
		for _, unit := range t.Units {
			unit.TeamID = t.ID
		}
		if err = db.Insert(ctx, &t.Units); err != nil {
			return err
		}
	}

	// Add all repositories to the team if it has access to all of them.
	if t.IncludesAllRepositories {
		err = addAllRepositories(ctx, t)
		if err != nil {
			return fmt.Errorf("addAllRepositories: %w", err)
		}
	}

	// Update organization number of teams.
	if _, err = db.Exec(ctx, "UPDATE `user` SET num_teams=num_teams+1 WHERE id = ?", t.OrgID); err != nil {
		return err
	}
	return committer.Commit()
}

// UpdateTeam updates information of team.
func UpdateTeam(t *organization.Team, authChanged, includeAllChanged bool) (err error) {
	if len(t.Name) == 0 {
		return util.NewInvalidArgumentErrorf("empty team name")
	}

	if len(t.Description) > 255 {
		t.Description = t.Description[:255]
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	t.LowerName = strings.ToLower(t.Name)
	has, err := sess.
		Where("org_id=?", t.OrgID).
		And("lower_name=?", t.LowerName).
		And("id!=?", t.ID).
		Get(new(organization.Team))
	if err != nil {
		return err
	} else if has {
		return organization.ErrTeamAlreadyExist{OrgID: t.OrgID, Name: t.LowerName}
	}

	if _, err = sess.ID(t.ID).Cols("name", "lower_name", "description",
		"can_create_org_repo", "authorize", "includes_all_repositories").Update(t); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	// update units for team
	if len(t.Units) > 0 {
		for _, unit := range t.Units {
			unit.TeamID = t.ID
		}
		// Delete team-unit.
		if _, err := sess.
			Where("team_id=?", t.ID).
			Delete(new(organization.TeamUnit)); err != nil {
			return err
		}
		if _, err = sess.Cols("org_id", "team_id", "type", "access_mode").Insert(&t.Units); err != nil {
			return err
		}
	}

	// Update access for team members if needed.
	if authChanged {
		if err = t.LoadRepositories(ctx); err != nil {
			return fmt.Errorf("LoadRepositories: %w", err)
		}

		for _, repo := range t.Repos {
			if err = access_model.RecalculateTeamAccesses(ctx, repo, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %w", err)
			}
		}
	}

	// Add all repositories to the team if it has access to all of them.
	if includeAllChanged && t.IncludesAllRepositories {
		err = addAllRepositories(ctx, t)
		if err != nil {
			return fmt.Errorf("addAllRepositories: %w", err)
		}
	}

	return committer.Commit()
}

// DeleteTeam deletes given team.
// It's caller's responsibility to assign organization ID.
func DeleteTeam(t *organization.Team) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := t.LoadRepositories(ctx); err != nil {
		return err
	}

	if err := t.LoadMembers(ctx); err != nil {
		return err
	}

	// update branch protections
	{
		protections := make([]*git_model.ProtectedBranch, 0, 10)
		err := db.GetEngine(ctx).In("repo_id",
			builder.Select("id").From("repository").Where(builder.Eq{"owner_id": t.OrgID})).
			Find(&protections)
		if err != nil {
			return fmt.Errorf("findProtectedBranches: %w", err)
		}
		for _, p := range protections {
			if err := git_model.RemoveTeamIDFromProtectedBranch(ctx, p, t.ID); err != nil {
				return err
			}
		}
	}

	if !t.IncludesAllRepositories {
		if err := removeAllRepositories(ctx, t); err != nil {
			return err
		}
	}

	if err := db.DeleteBeans(ctx,
		&organization.Team{ID: t.ID},
		&organization.TeamUser{OrgID: t.OrgID, TeamID: t.ID},
		&organization.TeamUnit{TeamID: t.ID},
		&organization.TeamInvite{TeamID: t.ID},
		&issues_model.Review{Type: issues_model.ReviewTypeRequest, ReviewerTeamID: t.ID}, // batch delete the binding relationship between team and PR (request review from team)
	); err != nil {
		return err
	}

	for _, tm := range t.Members {
		if err := removeInvalidOrgUser(ctx, tm.ID, t.OrgID); err != nil {
			return err
		}
	}

	// Update organization number of teams.
	if _, err := db.Exec(ctx, "UPDATE `user` SET num_teams=num_teams-1 WHERE id=?", t.OrgID); err != nil {
		return err
	}

	return committer.Commit()
}

// AddTeamMember adds new membership of given team to given organization,
// the user will have membership to given organization automatically when needed.
func AddTeamMember(team *organization.Team, userID int64) error {
	isAlreadyMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, userID)
	if err != nil || isAlreadyMember {
		return err
	}

	if err := organization.AddOrgUser(team.OrgID, userID); err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	// check in transaction
	isAlreadyMember, err = organization.IsTeamMember(ctx, team.OrgID, team.ID, userID)
	if err != nil || isAlreadyMember {
		return err
	}

	sess := db.GetEngine(ctx)

	if err := db.Insert(ctx, &organization.TeamUser{
		UID:    userID,
		OrgID:  team.OrgID,
		TeamID: team.ID,
	}); err != nil {
		return err
	} else if _, err := sess.Incr("num_members").ID(team.ID).Update(new(organization.Team)); err != nil {
		return err
	}

	team.NumMembers++

	// Give access to team repositories.
	// update exist access if mode become bigger
	subQuery := builder.Select("repo_id").From("team_repo").
		Where(builder.Eq{"team_id": team.ID})

	if _, err := sess.Where("user_id=?", userID).
		In("repo_id", subQuery).
		And("mode < ?", team.AccessMode).
		SetExpr("mode", team.AccessMode).
		Update(new(access_model.Access)); err != nil {
		return fmt.Errorf("update user accesses: %w", err)
	}

	// for not exist access
	var repoIDs []int64
	accessSubQuery := builder.Select("repo_id").From("access").Where(builder.Eq{"user_id": userID})
	if err := sess.SQL(subQuery.And(builder.NotIn("repo_id", accessSubQuery))).Find(&repoIDs); err != nil {
		return fmt.Errorf("select id accesses: %w", err)
	}

	accesses := make([]*access_model.Access, 0, 100)
	for i, repoID := range repoIDs {
		accesses = append(accesses, &access_model.Access{RepoID: repoID, UserID: userID, Mode: team.AccessMode})
		if (i%100 == 0 || i == len(repoIDs)-1) && len(accesses) > 0 {
			if err = db.Insert(ctx, accesses); err != nil {
				return fmt.Errorf("insert new user accesses: %w", err)
			}
			accesses = accesses[:0]
		}
	}

	if err := committer.Commit(); err != nil {
		return err
	}
	committer.Close()

	// this behaviour may spend much time so run it in a goroutine
	// FIXME: Update watch repos batchly
	if setting.Service.AutoWatchNewRepos {
		// Get team and its repositories.
		if err := team.LoadRepositories(db.DefaultContext); err != nil {
			log.Error("getRepositories failed: %v", err)
		}
		go func(repos []*repo_model.Repository) {
			for _, repo := range repos {
				if err = repo_model.WatchRepo(db.DefaultContext, userID, repo.ID, true); err != nil {
					log.Error("watch repo failed: %v", err)
				}
			}
		}(team.Repos)
	}

	return nil
}

func removeTeamMember(ctx context.Context, team *organization.Team, userID int64) error {
	e := db.GetEngine(ctx)
	isMember, err := organization.IsTeamMember(ctx, team.OrgID, team.ID, userID)
	if err != nil || !isMember {
		return err
	}

	// Check if the user to delete is the last member in owner team.
	if team.IsOwnerTeam() && team.NumMembers == 1 {
		return organization.ErrLastOrgOwner{UID: userID}
	}

	team.NumMembers--

	if err := team.LoadRepositories(ctx); err != nil {
		return err
	}

	if _, err := e.Delete(&organization.TeamUser{
		UID:    userID,
		OrgID:  team.OrgID,
		TeamID: team.ID,
	}); err != nil {
		return err
	} else if _, err = e.
		ID(team.ID).
		Cols("num_members").
		Update(team); err != nil {
		return err
	}

	// Delete access to team repositories.
	for _, repo := range team.Repos {
		if err := access_model.RecalculateUserAccess(ctx, repo, userID); err != nil {
			return err
		}

		// Remove watches from now unaccessible
		if err := reconsiderWatches(ctx, repo, userID); err != nil {
			return err
		}

		// Remove issue assignments from now unaccessible
		if err := reconsiderRepoIssuesAssignee(ctx, repo, userID); err != nil {
			return err
		}
	}

	return removeInvalidOrgUser(ctx, userID, team.OrgID)
}

func removeInvalidOrgUser(ctx context.Context, userID, orgID int64) error {
	// Check if the user is a member of any team in the organization.
	if count, err := db.GetEngine(ctx).Count(&organization.TeamUser{
		UID:   userID,
		OrgID: orgID,
	}); err != nil {
		return err
	} else if count == 0 {
		return removeOrgUser(ctx, orgID, userID)
	}
	return nil
}

// RemoveTeamMember removes member from given team of given organization.
func RemoveTeamMember(team *organization.Team, userID int64) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	if err := removeTeamMember(ctx, team, userID); err != nil {
		return err
	}
	return committer.Commit()
}
