// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
