// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	repo_model "code.gitea.io/gitea/models/repo"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRelease convert a repo_model.Release to api.Release
func ToRelease(r *repo_model.Release) *api.Release {
	attachments := make([]*api.Attachment, 0, len(r.Attachments))
	for _, attachment := range r.Attachments {
		attachments = append(attachments, ToAttachment(attachment))
	}
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
		Attachments:  attachments,
	}
}
