// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

// ToAPIRelease convert a repo_model.Release to api.Release
func ToAPIRelease(ctx context.Context, repo *repo_model.Repository, r *repo_model.Release) *api.Release {
	release := &api.Release{
		ID:           r.ID,
		TagName:      r.TagName,
		Target:       r.Target,
		Title:        r.Title,
		Note:         r.Note,
		URL:          r.APIURL(),
		HTMLURL:      r.HTMLURL(),
		TarURL:       r.TarURL(),
		ZipURL:       r.ZipURL(),
		UploadURL:    r.APIUploadURL(),
		IsDraft:      r.IsDraft,
		IsPrerelease: r.IsPrerelease,
		CreatedAt:    r.CreatedUnix.AsTime(),
		Publisher:    ToUser(ctx, r.Publisher, nil),
		Attachments:  ToAPIAttachments(repo, r.Attachments),
	}
	if !r.IsDraft {
		publishedAt := util.Iif(!r.PublishedUnix.IsZero(), r.PublishedUnix.AsTime(), r.CreatedUnix.AsTime())
		release.PublishedAt = &publishedAt
	}
	return release
}
