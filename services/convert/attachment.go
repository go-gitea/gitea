// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	repo_model "code.gitea.io/gitea/models/repo"
	api "code.gitea.io/gitea/modules/structs"
)

func WebAssetDownloadURL(repo *repo_model.Repository, attach *repo_model.Attachment) string {
	return attach.DownloadURL()
}

func APIAssetDownloadURL(repo *repo_model.Repository, attach *repo_model.Attachment) string {
	return attach.DownloadURL()
}

// ToAttachment converts models.Attachment to api.Attachment for API usage
func ToAttachment(repo *repo_model.Repository, a *repo_model.Attachment) *api.Attachment {
	return toAttachment(repo, a, WebAssetDownloadURL)
}

// ToAPIAttachment converts models.Attachment to api.Attachment for API usage
func ToAPIAttachment(repo *repo_model.Repository, a *repo_model.Attachment) *api.Attachment {
	return toAttachment(repo, a, APIAssetDownloadURL)
}

// toAttachment converts models.Attachment to api.Attachment for API usage
func toAttachment(repo *repo_model.Repository, a *repo_model.Attachment, getDownloadURL func(repo *repo_model.Repository, attach *repo_model.Attachment) string) *api.Attachment {
	return &api.Attachment{
		ID:            a.ID,
		Name:          a.Name,
		Created:       a.CreatedUnix.AsTime(),
		DownloadCount: a.DownloadCount,
		Size:          a.Size,
		UUID:          a.UUID,
		DownloadURL:   getDownloadURL(repo, a), // for web request json and api request json, return different download urls
	}
}

func ToAPIAttachments(repo *repo_model.Repository, attachments []*repo_model.Attachment) []*api.Attachment {
	return toAttachments(repo, attachments, APIAssetDownloadURL)
}

func toAttachments(repo *repo_model.Repository, attachments []*repo_model.Attachment, getDownloadURL func(repo *repo_model.Repository, attach *repo_model.Attachment) string) []*api.Attachment {
	converted := make([]*api.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		converted = append(converted, toAttachment(repo, attachment, getDownloadURL))
	}
	return converted
}
