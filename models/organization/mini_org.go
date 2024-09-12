// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// MinimalOrg represents a simple organization with only the needed columns
type MinimalOrg = Organization

// GetUserOrgsList returns all organizations the given user has access to
func GetUserOrgsList(ctx context.Context, user *user_model.User) ([]*MinimalOrg, error) {
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

	sess := db.GetEngine(ctx)
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
		Organization `xorm:"extends"`
		OrgCount     int
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
