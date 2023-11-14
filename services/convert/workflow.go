// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	user_model "code.gitea.io/gitea/models/user"
)

func ToWorkflowJob(ctx context.Context, doer *user_model.User, job *actions_model.ActionRunJob, run *actions_model.ActionRun) *WorkflowJob {
	return &WorkflowJob{
		Actor: &User{
			Identity: Identity{
				Name:  doer.Name,
				Email: doer.Email,
			},
			AvatarURL: doer.AvatarLink(),
			Login:     doer.Name,
			URL:       doer.HTMLURL(),
		},
		Commit: &Commit{},
	}
}

func ToWorkflowRun(ctx context.Context, doer *user_model.User, job *actions_model.ActionRunJob, run *actions_model.ActionRun) *WorkflowRun {
	return &WorkflowRun{
		Actor: &User{
			Identity: Identity{
				Name:  doer.Name,
				Email: doer.Email,
			},
			AvatarURL: doer.AvatarLink(),
			Login:     doer.Name,
			URL:       doer.HTMLURL(),
		},
		Commit: &Commit{},
	}
}
