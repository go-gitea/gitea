// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"net/url"

	git_model "code.gitea.io/gitea/models/git"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

// ToCommitStatus converts git_model.CommitStatus to api.CommitStatus
func ToCommitStatus(ctx context.Context, status *git_model.CommitStatus) *api.CommitStatus {
	apiStatus := &api.CommitStatus{
		Created:     status.CreatedUnix.AsTime(),
		Updated:     status.CreatedUnix.AsTime(),
		State:       status.State,
		TargetURL:   status.TargetURL,
		Description: status.Description,
		ID:          status.Index,
		URL:         status.APIURL(ctx),
		Context:     status.Context,
	}

	if status.CreatorID != 0 {
		creator, _ := user_model.GetUserByID(ctx, status.CreatorID)
		apiStatus.Creator = ToUser(ctx, creator, nil)
	}

	return apiStatus
}

func ToCommitStatuses(ctx context.Context, statuses []*git_model.CommitStatus) []*api.CommitStatus {
	apiStatuses := make([]*api.CommitStatus, len(statuses))
	for i, status := range statuses {
		apiStatuses[i] = ToCommitStatus(ctx, status)
	}
	return apiStatuses
}

// ToCombinedStatus converts List of CommitStatus to a CombinedStatus
func ToCombinedStatus(ctx context.Context, statuses []*git_model.CommitStatus, repo *api.Repository) *api.CombinedStatus {
	if len(statuses) == 0 {
		return nil
	}

	combinedStatus := git_model.CalcCommitStatus(statuses)

	return &api.CombinedStatus{
		State:      combinedStatus.State,
		Statuses:   ToCommitStatuses(ctx, statuses),
		SHA:        combinedStatus.SHA,
		TotalCount: len(statuses),
		Repository: repo,
		CommitURL:  repo.URL + "/commits/" + url.PathEscape(combinedStatus.SHA),
		URL:        repo.URL + "/commits/" + url.PathEscape(combinedStatus.SHA) + "/status",
	}
}
