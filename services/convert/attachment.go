// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	repo_model "code.gitea.io/gitea/models/repo"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAttachment converts models.Attachment to api.Attachment
func ToAttachment(a *repo_model.Attachment) *api.Attachment {
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

func ToAttachments(attachments []*repo_model.Attachment) []*api.Attachment {
	converted := make([]*api.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		converted = append(converted, ToAttachment(attachment))
	}
	return converted
}
