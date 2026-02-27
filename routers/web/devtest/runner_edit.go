// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/context"
)

func RunnerEdit(ctx *context.Context) {
	isDisabled := false
	switch ctx.PathParam("op") {
	case "disable":
		isDisabled = true
	case "enable":
		isDisabled = false
	}

	runner := &actions_model.ActionRunner{
		ID:          101,
		Name:        "devtest-runner-1",
		Version:     "1.3.0",
		Description: "Mock runner for devtest",
		AgentLabels: []string{"ubuntu-latest", "linux", "x64"},
		LastOnline:  timeutil.TimeStampNow(),
		LastActive:  timeutil.TimeStampNow() - 30,
		IsDisabled:  isDisabled,
	}

	tasks := []*actions_model.ActionTask{
		{
			ID:        9001,
			Status:    actions_model.StatusRunning,
			CommitSHA: "2ecfa6d0cb13ecf0af4f213f4f5d78c355f0d882",
		},
		{
			ID:        9000,
			Status:    actions_model.StatusSuccess,
			CommitSHA: "e3408f95f8f4ef6ef9f5d8477c0dced0f2e70f6a",
			Stopped:   timeutil.TimeStampNow() - 120,
		},
	}

	ctx.Data["Runner"] = runner
	ctx.Data["Tasks"] = tasks
	ctx.Data["Link"] = setting.AppSubURL + "/devtest/runner-edit"
	ctx.Data["Page"] = context.NewPagination(len(tasks), 30, 1, 5)
	ctx.HTML(http.StatusOK, "devtest/runner-edit")
}
