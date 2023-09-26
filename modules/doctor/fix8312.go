// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

func fixOwnerTeamCreateOrgRepo(ctx context.Context, logger log.Logger, autofix bool) error {
	count := 0

	err := db.Iterate(
		ctx,
		builder.Eq{"authorize": perm.AccessModeOwner, "can_create_org_repo": false},
		func(ctx context.Context, team *org_model.Team) error {
			team.CanCreateOrgRepo = true
			count++

			if !autofix {
				return nil
			}

			return models.UpdateTeam(ctx, team, false, false)
		},
	)
	if err != nil {
		logger.Critical("Unable to iterate across repounits to fix incorrect can_create_org_repo: Error %v", err)
		return err
	}

	if !autofix {
		if count == 0 {
			logger.Info("Found no team with incorrect can_create_org_repo")
		} else {
			logger.Warn("Found %d teams with incorrect can_create_org_repo", count)
		}
		return nil
	}
	logger.Info("Fixed %d teams with incorrect can_create_org_repo", count)

	return nil
}

func init() {
	Register(&Check{
		Title:     "Check for incorrect can_create_org_repo for org owner teams",
		Name:      "fix-owner-team-create-org-repo",
		IsDefault: false,
		Run:       fixOwnerTeamCreateOrgRepo,
		Priority:  7,
	})
}
