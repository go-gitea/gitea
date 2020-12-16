// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToCommitStatus converts models.CommitStatus to api.Status
func ToCommitStatus(status *models.CommitStatus) *api.Status {
	apiStatus := &api.Status{
		Created:     status.CreatedUnix.AsTime(),
		Updated:     status.CreatedUnix.AsTime(),
		State:       status.State,
		TargetURL:   status.TargetURL,
		Description: status.Description,
		ID:          status.Index,
		URL:         status.APIURL(),
		Context:     status.Context,
	}

	if status.CreatorID != 0 {
		creator, _ := models.GetUserByID(status.CreatorID)
		apiStatus.Creator = ToUser(creator, false, false)
	}

	return apiStatus
}
