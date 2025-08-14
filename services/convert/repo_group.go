// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

func ToAPIGroup(ctx context.Context, g *group_model.Group, actor *user_model.User) (*api.Group, error) {
	err := g.LoadAttributes(ctx)
	if err != nil {
		return nil, err
	}
	apiGroup := &api.Group{
		ID:            g.ID,
		Owner:         ToUser(ctx, g.Owner, actor),
		Name:          g.Name,
		Description:   g.Description,
		ParentGroupID: g.ParentGroupID,
		Link:          g.GroupLink(),
		SortOrder:     g.SortOrder,
	}
	if apiGroup.NumSubgroups, err = group_model.CountGroups(ctx, &group_model.FindGroupsOptions{
		ParentGroupID: g.ID,
	}); err != nil {
		return nil, err
	}
	if _, apiGroup.NumRepos, err = repo_model.SearchRepositoryByCondition(ctx, repo_model.SearchRepoOptions{
		GroupID: g.ID,
		Actor:   actor,
		OwnerID: g.OwnerID,
	}, repo_model.AccessibleRepositoryCondition(actor, unit.TypeInvalid), true); err != nil {
		return nil, err
	}
	return apiGroup, nil
}
