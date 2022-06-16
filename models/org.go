// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// MinimalOrg represents a simple orgnization with only needed columns
type MinimalOrg = organization.Organization

// GetUserOrgsList returns one user's all orgs list
func GetUserOrgsList(user *user_model.User) ([]*MinimalOrg, error) {
	schema, err := db.TableInfo(new(user_model.User))
	if err != nil {
		return nil, err
	}

	outputCols := []string{
		"id",
		"name",
		"full_name",
		"visibility",
		"avatar",
		"avatar_email",
		"use_custom_avatar",
	}

	groupByCols := &strings.Builder{}
	for _, col := range outputCols {
		fmt.Fprintf(groupByCols, "`%s`.%s,", schema.Name, col)
	}
	groupByStr := groupByCols.String()
	groupByStr = groupByStr[0 : len(groupByStr)-1]

	sess := db.GetEngine(db.DefaultContext)
	sess = sess.Select(groupByStr+", count(distinct repo_id) as org_count").
		Table("user").
		Join("INNER", "team", "`team`.org_id = `user`.id").
		Join("INNER", "team_user", "`team`.id = `team_user`.team_id").
		Join("LEFT", builder.
			Select("id as repo_id, owner_id as repo_owner_id").
			From("repository").
			Where(repo_model.AccessibleRepositoryCondition(user, unit.TypeInvalid)), "`repository`.repo_owner_id = `team`.org_id").
		Where("`team_user`.uid = ?", user.ID).
		GroupBy(groupByStr)

	type OrgCount struct {
		organization.Organization `xorm:"extends"`
		OrgCount                  int
	}

	orgCounts := make([]*OrgCount, 0, 10)

	if err := sess.
		Asc("`user`.name").
		Find(&orgCounts); err != nil {
		return nil, err
	}

	orgs := make([]*MinimalOrg, len(orgCounts))
	for i, orgCount := range orgCounts {
		orgCount.Organization.NumRepos = orgCount.OrgCount
		orgs[i] = &orgCount.Organization
	}

	return orgs, nil
}

func removeOrgUser(ctx context.Context, orgID, userID int64) error {
	ou := new(organization.OrgUser)

	sess := db.GetEngine(ctx)

	has, err := sess.
		Where("uid=?", userID).
		And("org_id=?", orgID).
		Get(ou)
	if err != nil {
		return fmt.Errorf("get org-user: %v", err)
	} else if !has {
		return nil
	}

	org, err := organization.GetOrgByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("GetUserByID [%d]: %v", orgID, err)
	}

	// Check if the user to delete is the last member in owner team.
	if isOwner, err := organization.IsOrganizationOwner(ctx, orgID, userID); err != nil {
		return err
	} else if isOwner {
		t, err := organization.GetOwnerTeam(ctx, org.ID)
		if err != nil {
			return err
		}
		if t.NumMembers == 1 {
			if err := t.GetMembersCtx(ctx); err != nil {
				return err
			}
			if t.Members[0].ID == userID {
				return organization.ErrLastOrgOwner{UID: userID}
			}
		}
	}

	if _, err := sess.ID(ou.ID).Delete(ou); err != nil {
		return err
	} else if _, err = db.Exec(ctx, "UPDATE `user` SET num_members=num_members-1 WHERE id=?", orgID); err != nil {
		return err
	}

	// Delete all repository accesses and unwatch them.
	env, err := organization.AccessibleReposEnv(ctx, org, userID)
	if err != nil {
		return fmt.Errorf("AccessibleReposEnv: %v", err)
	}
	repoIDs, err := env.RepoIDs(1, org.NumRepos)
	if err != nil {
		return fmt.Errorf("GetUserRepositories [%d]: %v", userID, err)
	}
	for _, repoID := range repoIDs {
		if err = repo_model.WatchRepo(ctx, userID, repoID, false); err != nil {
			return err
		}
	}

	if len(repoIDs) > 0 {
		if _, err = sess.
			Where("user_id = ?", userID).
			In("repo_id", repoIDs).
			Delete(new(access_model.Access)); err != nil {
			return err
		}
	}

	// Delete member in his/her teams.
	teams, err := organization.GetUserOrgTeams(ctx, org.ID, userID)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if err = removeTeamMember(ctx, t, userID); err != nil {
			return err
		}
	}

	return nil
}

// RemoveOrgUser removes user from given organization.
func RemoveOrgUser(orgID, userID int64) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	if err := removeOrgUser(ctx, orgID, userID); err != nil {
		return err
	}
	return committer.Commit()
}
