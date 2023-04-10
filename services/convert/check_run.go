// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	git_model "code.gitea.io/gitea/models/git"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

// ToCommitStatus converts git_model.CheckRun to api.CheckRun
func ToChekckRun(ctx context.Context, checkRun *git_model.CheckRun) *api.CheckRun {
	status := checkRun.Status.ToAPI()

	apiCheckRun := &api.CheckRun{
		ID:         checkRun.ID,
		NodeID:     checkRun.NameHash,
		HeadSHA:    checkRun.HeadSHA,
		ExternalID: &checkRun.ExternalID,
		DetailsURL: &checkRun.DetailsURL,
		Status:     &status,
		Name:       checkRun.Name,
	}

	if checkRun.Status == git_model.CheckRunStatusCompleted {
		conclusion := checkRun.Conclusion.ToAPI()
		apiCheckRun.Conclusion = &conclusion
	}

	if checkRun.StartedAt != 0 {
		time := checkRun.StartedAt.AsTime()
		apiCheckRun.StartedAt = &time
	}

	if checkRun.CompletedAt != 0 {
		time := checkRun.CompletedAt.AsTime()
		apiCheckRun.CompletedAt = &time
	}

	if checkRun.ID != 0 {
		creator, _ := user_model.GetUserByID(ctx, checkRun.CreatorID)
		apiCheckRun.Creator = ToUser(ctx, creator, nil)
	}

	if checkRun.Output != nil {
		apiCheckRun.Output = &api.CheckRunOutput{
			Annotations: checkRun.Output.Annotations,
		}

		if len(checkRun.Output.Title) > 0 {
			apiCheckRun.Output.Title = &checkRun.Output.Title
		}

		if len(checkRun.Output.Summary) > 0 {
			apiCheckRun.Output.Summary = &checkRun.Output.Summary
		}

		if len(checkRun.Output.Text) > 0 {
			apiCheckRun.Output.Text = &checkRun.Output.Text
		}

		if len(checkRun.Output.AnnotationsURL) > 0 {
			apiCheckRun.Output.AnnotationsURL = &checkRun.Output.AnnotationsURL
		}
	}

	return apiCheckRun
}
