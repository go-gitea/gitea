// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	repo_model "code.gitea.io/gitea/models/repo"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRelease convert a repo_model.Release to api.Release
func ToRelease(r *repo_model.Release) *api.Release {
	return &api.Release{
		ID:           r.ID,
		TagName:      r.TagName,
		Target:       r.Target,
		Title:        r.Title,
		Note:         r.Note,
		URL:          r.APIURL(),
		HTMLURL:      r.HTMLURL(),
		TarURL:       r.TarURL(),
		ZipURL:       r.ZipURL(),
		IsDraft:      r.IsDraft,
		IsPrerelease: r.IsPrerelease,
		CreatedAt:    r.CreatedUnix.AsTime(),
		PublishedAt:  r.CreatedUnix.AsTime(),
		Publisher:    ToUser(r.Publisher, nil),
		Attachments:  ToAttachments(r.Attachments),
	}
}
