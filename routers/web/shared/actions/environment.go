// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	actions_model "gitea.dev/models/actions"
	"gitea.dev/services/context"
)

// EnvironmentAssignment loads the environment named by an environment settings route
func EnvironmentAssignment(ctx *context.Context) {
	env, err := actions_model.GetEnvironmentByRepoAndName(ctx, ctx.Repo.Repository.ID, ctx.PathParam("environment_name"))
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Data["Environment"] = env
}

// EnvironmentFromContext returns what EnvironmentAssignment loaded, nil on any other route.
func EnvironmentFromContext(ctx *context.Context) *actions_model.ActionEnvironment {
	env, _ := ctx.Data["Environment"].(*actions_model.ActionEnvironment)
	return env
}
