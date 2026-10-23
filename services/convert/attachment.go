// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
)

func WebAssetDownloadURL(ctx context.Context, repo *repo_model.Repository, attach *repo_model.Attachment) string {
	return attach.DownloadURL(ctx)
}

func APIAssetDownloadURL(ctx context.Context, repo *repo_model.Repository, attach *repo_model.Attachment) string {
	return attach.DownloadURL(ctx)
}

// ToAttachment converts models.Attachment to api.Attachment for API usage
func ToAttachment(ctx context.Context, repo *repo_model.Repository, a *repo_model.Attachment) *api.Attachment {
	return toAttachment(ctx, repo, a, WebAssetDownloadURL)
}

// ToAPIAttachment converts models.Attachment to api.Attachment for API usage
func ToAPIAttachment(ctx context.Context, repo *repo_model.Repository, a *repo_model.Attachment) *api.Attachment {
	return toAttachment(ctx, repo, a, APIAssetDownloadURL)
}

// toAttachment converts models.Attachment to api.Attachment for API usage
func toAttachment(ctx context.Context, repo *repo_model.Repository, a *repo_model.Attachment, getDownloadURL func(ctx context.Context, repo *repo_model.Repository, attach *repo_model.Attachment) string) *api.Attachment {
	return &api.Attachment{
		ID:            a.ID,
		Name:          a.Name,
		Created:       a.CreatedUnix.AsTime(),
		DownloadCount: a.DownloadCount,
		Size:          a.Size,
		UUID:          a.UUID,
		DownloadURL:   getDownloadURL(ctx, repo, a), // for web/api requests, return different download URLs
	}
}

func ToAPIAttachments(ctx context.Context, repo *repo_model.Repository, attachments []*repo_model.Attachment) []*api.Attachment {
	return toAttachments(ctx, repo, attachments, APIAssetDownloadURL)
}

func toAttachments(ctx context.Context, repo *repo_model.Repository, attachments []*repo_model.Attachment, getDownloadURL func(ctx context.Context, repo *repo_model.Repository, attach *repo_model.Attachment) string) []*api.Attachment {
	converted := make([]*api.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		converted = append(converted, toAttachment(ctx, repo, attachment, getDownloadURL))
	}
	return converted
}
