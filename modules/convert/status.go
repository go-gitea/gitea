// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	git_model "code.gitea.io/gitea/models/git"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

// ToCommitStatus converts git_model.CommitStatus to api.CommitStatus
func ToCommitStatus(status *git_model.CommitStatus) *api.CommitStatus {
	apiStatus := &api.CommitStatus{
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
		creator, _ := user_model.GetUserByID(status.CreatorID)
		apiStatus.Creator = ToUser(creator, nil)
	}

	return apiStatus
}

// ToCombinedStatus converts List of CommitStatus to a CombinedStatus
func ToCombinedStatus(statuses []*git_model.CommitStatus, repo *api.Repository) *api.CombinedStatus {
	if len(statuses) == 0 {
		return nil
	}

	retStatus := &api.CombinedStatus{
		SHA:        statuses[0].SHA,
		TotalCount: len(statuses),
		Repository: repo,
		URL:        "",
	}

	retStatus.Statuses = make([]*api.CommitStatus, 0, len(statuses))
	for _, status := range statuses {
		retStatus.Statuses = append(retStatus.Statuses, ToCommitStatus(status))
		if status.State.NoBetterThan(retStatus.State) {
			retStatus.State = status.State
		}
	}

	return retStatus
}
