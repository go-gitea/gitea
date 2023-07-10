// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"strconv"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

func AssetDownloadURL(repo *repo_model.Repository, attach *repo_model.Attachment) string {
	if attach.CustomDownloadURL != "" {
		return attach.CustomDownloadURL
	}

	// /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id}
	return setting.AppURL + "api/repos/" + repo.FullName() + "/releases/" + strconv.FormatInt(attach.ReleaseID, 10) + "/assets/" + strconv.FormatInt(attach.ID, 10)
}

// ToAttachment converts models.Attachment to api.Attachment
func ToAttachment(repo *repo_model.Repository, a *repo_model.Attachment) *api.Attachment {
	return &api.Attachment{
		ID:            a.ID,
		Name:          a.Name,
		Created:       a.CreatedUnix.AsTime(),
		DownloadCount: a.DownloadCount,
		Size:          a.Size,
		UUID:          a.UUID,
		DownloadURL:   a.DownloadURL(),
	}
}

// ToAPIAttachment converts models.Attachment to api.Attachment for API usage
func ToAPIAttachment(repo *repo_model.Repository, a *repo_model.Attachment) *api.Attachment {
	return &api.Attachment{
		ID:            a.ID,
		Name:          a.Name,
		Created:       a.CreatedUnix.AsTime(),
		DownloadCount: a.DownloadCount,
		Size:          a.Size,
		UUID:          a.UUID,
		DownloadURL:   AssetDownloadURL(repo, a),
	}
}

func ToAPIAttachments(repo *repo_model.Repository, attachments []*repo_model.Attachment) []*api.Attachment {
	converted := make([]*api.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		converted = append(converted, ToAPIAttachment(repo, attachment))
	}
	return converted
}
