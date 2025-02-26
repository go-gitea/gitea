// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"fmt"
	mathRand "math/rand/v2"
	"net/http"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/repo/actions"
	"code.gitea.io/gitea/services/context"
)

func generateMockStepsLog(logCur actions.LogCursor) (stepsLog []*actions.ViewStepLog) {
	mockedLogs := []string{
		"::group::test group for: step={step}, cursor={cursor}",
		"in group msg for: step={step}, cursor={cursor}",
		"in group msg for: step={step}, cursor={cursor}",
		"in group msg for: step={step}, cursor={cursor}",
		"::endgroup::",
		"message for: step={step}, cursor={cursor}",
		"message for: step={step}, cursor={cursor}",
		"##[group]test group for: step={step}, cursor={cursor}",
		"in group msg for: step={step}, cursor={cursor}",
		"##[endgroup]",
	}
	cur := logCur.Cursor // usually the cursor is the "file offset", but here we abuse it as "line number" to make the mock easier, intentionally
	mockCount := util.Iif(logCur.Step == 0, 3, 1)
	if logCur.Step == 1 && logCur.Cursor == 0 {
		mockCount = 30 // for the first batch, return as many as possible to test the auto-expand and auto-scroll
	}
	for i := 0; i < mockCount; i++ {
		logStr := mockedLogs[int(cur)%len(mockedLogs)]
		cur++
		logStr = strings.ReplaceAll(logStr, "{step}", fmt.Sprintf("%d", logCur.Step))
		logStr = strings.ReplaceAll(logStr, "{cursor}", fmt.Sprintf("%d", cur))
		stepsLog = append(stepsLog, &actions.ViewStepLog{
			Step:    logCur.Step,
			Cursor:  cur,
			Started: time.Now().Unix() - 1,
			Lines: []*actions.ViewStepLogLine{
				{Index: cur, Message: logStr, Timestamp: float64(time.Now().UnixNano()) / float64(time.Second)},
			},
		})
	}
	return stepsLog
}

func MockActionsView(ctx *context.Context) {
	ctx.Data["RunID"] = ctx.PathParam("run")
	ctx.Data["JobID"] = ctx.PathParam("job")
	ctx.HTML(http.StatusOK, "devtest/repo-action-view")
}

func MockActionsRunsJobs(ctx *context.Context) {
	runID := ctx.PathParamInt64("run")

	req := web.GetForm(ctx).(*actions.ViewRequest)
	resp := &actions.ViewResponse{}
	resp.State.Run.TitleHTML = `mock run title <a href="/">link</a>`
	resp.State.Run.Status = actions_model.StatusRunning.String()
	resp.State.Run.CanCancel = runID == 10
	resp.State.Run.CanApprove = runID == 20
	resp.State.Run.CanRerun = runID == 30
	resp.State.Run.CanDeleteArtifact = true
	resp.State.Run.WorkflowID = "workflow-id"
	resp.State.Run.WorkflowLink = "./workflow-link"
	resp.State.Run.Commit = actions.ViewCommit{
		ShortSha: "ccccdddd",
		Link:     "./commit-link",
		Pusher: actions.ViewUser{
			DisplayName: "pusher user",
			Link:        "./pusher-link",
		},
		Branch: actions.ViewBranch{
			Name:      "commit-branch",
			Link:      "./branch-link",
			IsDeleted: false,
		},
	}
	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:   "artifact-a",
		Size:   100 * 1024,
		Status: "expired",
	})
	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:   "artifact-b",
		Size:   1024 * 1024,
		Status: "completed",
	})

	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID * 10,
		Name:     "job 100",
		Status:   actions_model.StatusRunning.String(),
		CanRerun: true,
		Duration: "1h",
	})
	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID*10 + 1,
		Name:     "job 101",
		Status:   actions_model.StatusWaiting.String(),
		CanRerun: false,
		Duration: "2h",
	})
	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID*10 + 2,
		Name:     "job 102",
		Status:   actions_model.StatusFailure.String(),
		CanRerun: false,
		Duration: "3h",
	})

	resp.State.CurrentJob.Steps = append(resp.State.CurrentJob.Steps, &actions.ViewJobStep{
		Summary:  "step 0 (mock slow)",
		Duration: time.Hour.String(),
		Status:   actions_model.StatusRunning.String(),
	})
	resp.State.CurrentJob.Steps = append(resp.State.CurrentJob.Steps, &actions.ViewJobStep{
		Summary:  "step 1 (mock fast)",
		Duration: time.Hour.String(),
		Status:   actions_model.StatusRunning.String(),
	})
	resp.State.CurrentJob.Steps = append(resp.State.CurrentJob.Steps, &actions.ViewJobStep{
		Summary:  "step 2 (mock error)",
		Duration: time.Hour.String(),
		Status:   actions_model.StatusRunning.String(),
	})
	if len(req.LogCursors) == 0 {
		ctx.JSON(http.StatusOK, resp)
		return
	}

	resp.Logs.StepsLog = []*actions.ViewStepLog{}
	doSlowResponse := false
	doErrorResponse := false
	for _, logCur := range req.LogCursors {
		if !logCur.Expanded {
			continue
		}
		doSlowResponse = doSlowResponse || logCur.Step == 0
		doErrorResponse = doErrorResponse || logCur.Step == 2
		resp.Logs.StepsLog = append(resp.Logs.StepsLog, generateMockStepsLog(logCur)...)
	}
	if doErrorResponse {
		if mathRand.Float64() > 0.5 {
			ctx.HTTPError(http.StatusInternalServerError, "devtest mock error response")
			return
		}
	}
	if doSlowResponse {
		time.Sleep(time.Duration(3000) * time.Millisecond)
	} else {
		time.Sleep(time.Duration(100) * time.Millisecond) // actually, frontend reload every 1 second, any smaller delay is fine
	}
	ctx.JSON(http.StatusOK, resp)
}
