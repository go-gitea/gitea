// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

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

// ToCombinedStatus converts List of CommitStatus to a CombinedStatus
func ToCombinedStatus(ctx context.Context, statuses []*git_model.CommitStatus, repo *api.Repository) *api.CombinedStatus {
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
		retStatus.Statuses = append(retStatus.Statuses, ToCommitStatus(ctx, status))
		if retStatus.State == "" || status.State.NoBetterThan(retStatus.State) {
			retStatus.State = status.State
		}
	}
	// According to https://docs.github.com/en/rest/commits/statuses?apiVersion=2022-11-28#get-the-combined-status-for-a-specific-reference
	// > Additionally, a combined state is returned. The state is one of:
	// > failure if any of the contexts report as error or failure
	// > pending if there are no statuses or a context is pending
	// > success if the latest status for all contexts is success
	if retStatus.State.IsError() {
		retStatus.State = api.CommitStatusFailure
	}

	return retStatus
}
