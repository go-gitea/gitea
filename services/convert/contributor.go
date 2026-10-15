// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
)

// ToContributor converts a repo_model.RepoContributor with its user loaded to api.Contributor
func ToContributor(ctx context.Context, c *repo_model.RepoContributor) *api.Contributor {
	contributor := &api.Contributor{Contributions: c.Commits, Additions: c.Additions, Deletions: c.Deletions}
	if c.User != nil {
		contributor.Type = "User"
		contributor.ID, contributor.Login = c.User.ID, c.User.Name
		contributor.AvatarURL, contributor.HTMLURL = c.User.AvatarLink(ctx), c.User.HTMLURL(ctx)
	} else {
		contributor.Type = "Anonymous"
		contributor.Name, contributor.Email = c.Name, c.Email
	}
	return contributor
}
