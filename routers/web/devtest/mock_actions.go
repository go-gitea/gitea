// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"archive/zip"
	"fmt"
	"io"
	mathRand "math/rand/v2"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/repo/actions"
	"code.gitea.io/gitea/services/context"
)

type mockArtifactFile struct {
	Path    string
	Content string
}

var mockActionsArtifactFiles = map[string][]mockArtifactFile{
	"artifact-b": {
		{
			Path:    "report.txt",
			Content: "artifact-b report",
		},
	},
	"artifact-lcov-coverage": {
		{
			Path: "coverage/index.html",
			// Realistic genhtml-style report. CSS is inlined because the raw
			// handler sniffs external .css files as text/plain, which browsers
			// refuse to apply as a stylesheet. JS is intentionally blocked by
			// the iframe sandbox="" (no allow-scripts).
			Content: `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<title>LCOV - Coverage Report</title>
<style>
body{font-family:sans-serif;font-size:14px;margin:1em}
table{border-collapse:collapse;width:100%}
th,td{padding:4px 8px;text-align:left;border:1px solid #ccc}
th{background:#4a90d9;color:#fff;cursor:pointer}
.hi{background:#cfc}.med{background:#ffc}.lo{background:#fcc}
.bar{display:inline-block;height:12px;background:#4a90d9}
.pct{min-width:3em;text-align:right}
#sort-note{color:#888;font-size:12px;margin-top:.5em}
</style>
<script>
// Column sorting — blocked by iframe sandbox="" (no allow-scripts)
document.addEventListener("DOMContentLoaded", function() {
  document.getElementById("sort-note").textContent = "JS loaded: sorting enabled";
  document.querySelectorAll("th[data-col]").forEach(function(th) {
    th.addEventListener("click", function() {
      var col = parseInt(th.getAttribute("data-col"));
      var tbody = document.querySelector("tbody");
      var rows = Array.from(tbody.querySelectorAll("tr"));
      rows.sort(function(a, b) {
        return a.cells[col].textContent.localeCompare(b.cells[col].textContent, undefined, {numeric: true});
      });
      rows.forEach(function(r) { tbody.appendChild(r); });
    });
  });
});
</script>
</head>
<body>
<h2>Coverage Report</h2>
<table>
  <thead>
    <tr>
      <th>Directory/File</th>
      <th>Lines</th>
      <th>Coverage</th>
    </tr>
  </thead>
  <tbody>
    <tr><td>src/</td><td></td><td></td></tr>
    <tr class="hi"><td>&nbsp;&nbsp;<a href="main.go.html">main.go</a></td><td>120 / 132</td><td>90.9 %</td></tr>
    <tr class="med"><td>&nbsp;&nbsp;<a href="util.go.html">util.go</a></td><td>45 / 60</td><td>75.0 %</td></tr>
    <tr class="lo"><td>&nbsp;&nbsp;<a href="legacy.go.html">legacy.go</a></td><td>12 / 38</td><td>31.6 %</td></tr>
    <tr><th colspan="3">Total: 177 / 230 &mdash; 76.9 %</th></tr>
  </tbody>
</table>
<p id="sort-note">(JS sandboxed — column sorting disabled)</p>
</body>
</html>`,
		},
		{
			Path:    "coverage/lcov.info",
			Content: "TN:\nSF:src/main.go\nDA:1,1\nDA:2,1\nDA:10,0\nend_of_record\nTN:\nSF:src/util.go\nDA:1,1\nDA:5,0\nend_of_record\n",
		},
		{
			Path: "coverage/main.go.html",
			Content: `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"/><title>main.go - Coverage</title>
<style>
body{font-family:monospace;font-size:13px}
.hit{background:#cfc}.miss{background:#fcc}.neutral{background:#eee}
td:first-child{color:#888;text-align:right;padding-right:8px;user-select:none}
</style>
</head>
<body>
<h3>src/main.go</h3>
<table>
<tr><td>1</td><td class="hit">func main() {</td></tr>
<tr><td>2</td><td class="hit">  fmt.Println("hello")</td></tr>
<tr><td>3</td><td class="neutral">}</td></tr>
<tr><td>10</td><td class="miss">  unusedFunc()</td></tr>
</table>
</body>
</html>`,
		},
	},
	"artifact-really-loooooooooooooooooooooooooooooooooooooooooooooooooooooooong": {
		{
			Path:    "index.html",
			Content: "<html><body>mock preview</body></html>",
		},
		{
			Path:    "logs/output.txt",
			Content: "mock logs",
		},
	},
}

func mockArtifactFilePaths(files []mockArtifactFile) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return paths
}

type generateMockStepsLogOptions struct {
	mockCountFirst   int
	mockCountGeneral int
	groupRepeat      int
}

func generateMockStepsLog(logCur actions.LogCursor, opts generateMockStepsLogOptions) (stepsLog []*actions.ViewStepLog) {
	var mockedLogs []string
	mockedLogs = append(mockedLogs, "::group::test group for: step={step}, cursor={cursor}")
	mockedLogs = append(mockedLogs, slices.Repeat([]string{"in group msg for: step={step}, cursor={cursor}"}, opts.groupRepeat)...)
	mockedLogs = append(mockedLogs, "::endgroup::")
	mockedLogs = append(mockedLogs,
		"message for: step={step}, cursor={cursor}",
		"message for: step={step}, cursor={cursor}",
		"##[group]test group for: step={step}, cursor={cursor}",
		"in group msg for: step={step}, cursor={cursor}",
		"##[endgroup]",
		"::error::mock error for: step={step}, cursor={cursor}",
		"::warning::mock warning for: step={step}, cursor={cursor}",
		"::notice::mock notice for: step={step}, cursor={cursor}",
		"::debug::mock debug for: step={step}, cursor={cursor}",
	)
	// usually the cursor is the "file offset", but here we abuse it as "line number" to make the mock easier, intentionally
	cur := logCur.Cursor
	// for the first batch, return as many as possible to test the auto-expand and auto-scroll
	mockCount := util.Iif(logCur.Cursor == 0, opts.mockCountFirst, opts.mockCountGeneral)
	for range mockCount {
		logStr := mockedLogs[int(cur)%len(mockedLogs)]
		cur++
		logStr = strings.ReplaceAll(logStr, "{step}", strconv.Itoa(logCur.Step))
		logStr = strings.ReplaceAll(logStr, "{cursor}", strconv.FormatInt(cur, 10))
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
	if runID := ctx.PathParamInt64("run"); runID == 0 {
		ctx.Redirect("/repo-action-view/runs/10")
		return
	}
	ctx.Data["JobID"] = ctx.PathParamInt64("job")
	ctx.Data["ActionsViewURL"] = ctx.Req.URL.Path
	ctx.HTML(http.StatusOK, "devtest/repo-action-view")
}

func MockActionsRunsJobs(ctx *context.Context) {
	runID := ctx.PathParamInt64("run")
	attemptID := ctx.PathParamInt64("attempt")

	alignTime := func(v, unit int64) int64 {
		return (v + unit) / unit * unit
	}
	resp := &actions.ViewResponse{}
	resp.State.Run.RepoID = 12345
	resp.State.Run.TitleHTML = `mock run title <a href="/">link</a>`
	resp.State.Run.Link = setting.AppSubURL + "/devtest/repo-action-view/runs/" + strconv.FormatInt(runID, 10)
	resp.State.Run.CanDeleteArtifact = true
	resp.State.Run.WorkflowID = "workflow-id"
	resp.State.Run.WorkflowLink = "./workflow-link"
	resp.State.Run.TriggerEvent = "push"
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
	now := time.Now()
	currentAttemptNum := int64(1)
	if attemptID > 0 {
		currentAttemptNum = attemptID
	}
	user2 := &user_model.User{Name: "user2"}
	user3 := &user_model.User{Name: "user3"}
	attempts := []*actions_model.ActionRunAttempt{{
		Attempt:       1,
		Status:        actions_model.StatusSuccess,
		Created:       timeutil.TimeStamp(now.Add(-time.Hour).Unix()),
		TriggerUserID: 2,
		TriggerUser:   user2,
	}}
	if runID == 10 {
		attempts = []*actions_model.ActionRunAttempt{
			{
				Attempt:       3,
				Status:        actions_model.StatusSuccess,
				Created:       timeutil.TimeStamp(alignTime(now.Add(-time.Hour).Unix(), 3600)),
				TriggerUserID: 2,
				TriggerUser:   user2,
			},
			{
				Attempt:       2,
				Status:        actions_model.StatusFailure,
				Created:       timeutil.TimeStamp(alignTime(now.Add(-2*time.Hour).Unix(), 3600)),
				TriggerUserID: 1,
				TriggerUser:   user3,
			},
			{
				Attempt:       1,
				Status:        actions_model.StatusSuccess,
				Created:       timeutil.TimeStamp(alignTime(now.Add(-3*time.Hour).Unix(), 3600)),
				TriggerUserID: 2,
				TriggerUser:   user2,
			},
		}
		if attemptID == 0 {
			currentAttemptNum = 3
		}
	}

	latestAttempt := attempts[0]
	resp.State.Run.RunAttempt = currentAttemptNum
	resp.State.Run.Done = latestAttempt.Status.IsDone()
	resp.State.Run.Status = latestAttempt.Status.String()
	resp.State.Run.Duration = "1h 23m 45s"
	resp.State.Run.TriggeredAt = latestAttempt.Created.AsTime().Unix()
	resp.State.Run.ViewLink = resp.State.Run.Link
	for _, attempt := range attempts {
		link := resp.State.Run.Link
		if attempt.Attempt != latestAttempt.Attempt {
			link = fmt.Sprintf("%s/attempts/%d", resp.State.Run.Link, attempt.Attempt)
		}
		current := attempt.Attempt == currentAttemptNum
		if current {
			resp.State.Run.Status = attempt.Status.String()
			resp.State.Run.Done = attempt.Status.IsDone()
			resp.State.Run.TriggeredAt = attempt.Created.AsTime().Unix()
			if attempt.Attempt != latestAttempt.Attempt {
				resp.State.Run.ViewLink = link
			}
		}
		resp.State.Run.Attempts = append(resp.State.Run.Attempts, &actions.ViewRunAttempt{
			Attempt:         attempt.Attempt,
			Status:          attempt.Status.String(),
			Done:            attempt.Status.IsDone(),
			Link:            link,
			Current:         current,
			Latest:          attempt.Attempt == latestAttempt.Attempt,
			TriggeredAt:     attempt.Created.AsTime().Unix(),
			TriggerUserName: attempt.TriggerUser.GetDisplayName(),
			TriggerUserLink: attempt.TriggerUser.HomeLink(),
		})
	}
	isLatestAttempt := currentAttemptNum == latestAttempt.Attempt
	resp.State.Run.CanCancel = runID == 10 && isLatestAttempt
	resp.State.Run.CanApprove = runID == 20 && isLatestAttempt
	resp.State.Run.CanRerun = runID == 30 && isLatestAttempt
	resp.State.Run.CanRerunFailed = runID == 30 && isLatestAttempt

	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:        "artifact-a",
		Size:        100 * 1024,
		Status:      "expired",
		ExpiresUnix: alignTime(time.Now().Add(-24*time.Hour).Unix(), 3600),
	})
	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:        "artifact-b",
		Size:        1024 * 1024,
		Status:      "completed",
		ExpiresUnix: alignTime(time.Now().Add(24*time.Hour).Unix(), 3600),
	})
	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:        "artifact-very-loooooooooooooooooooooooooooooooooooooooooooooooooooooooong",
		Size:        100 * 1024,
		Status:      "expired",
		ExpiresUnix: alignTime(time.Now().Add(-24*time.Hour).Unix(), 3600),
	})
	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:        "artifact-lcov-coverage",
		Size:        256 * 1024,
		Status:      "completed",
		ExpiresUnix: alignTime(time.Now().Add(24*time.Hour).Unix(), 3600),
	})
	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:        "artifact-really-loooooooooooooooooooooooooooooooooooooooooooooooooooooooong",
		Size:        1024 * 1024,
		Status:      "completed",
		ExpiresUnix: 0,
	})

	jobLink := func(jobID int64) string {
		return fmt.Sprintf("%s/jobs/%d", resp.State.Run.Link, jobID)
	}

	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID * 10,
		Link:     jobLink(runID * 10),
		JobID:    "job-100",
		Name:     "job 100 (testsubname)",
		Status:   actions_model.StatusRunning.String(),
		CanRerun: true,
		Duration: "1h23m45s",
	})
	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID*10 + 1,
		Link:     jobLink(runID*10 + 1),
		JobID:    "job-101",
		Name:     "job 101",
		Status:   actions_model.StatusWaiting.String(),
		CanRerun: false,
		Duration: "2h",
		Needs:    []string{"job-100"},
	})
	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID*10 + 2,
		Link:     jobLink(runID*10 + 2),
		JobID:    "job-102",
		Name:     "ULTRA LOOOOOOOOOOOONG job name 102 that exceeds the limit",
		Status:   actions_model.StatusFailure.String(),
		CanRerun: false,
		Duration: "3h",
		Needs:    []string{"job-100", "job-101"},
	})
	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID*10 + 3,
		Link:     jobLink(runID*10 + 3),
		JobID:    "job-103",
		Name:     "job 103",
		Status:   actions_model.StatusCancelled.String(),
		CanRerun: false,
		Duration: "2m",
		Needs:    []string{"job-100"},
	})

	// add more jobs to a run for UI testing
	if resp.State.Run.CanCancel {
		for i := range 10 {
			jobID := runID*1000 + int64(i)
			resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
				ID:       jobID,
				Link:     jobLink(jobID),
				JobID:    "job-dup-test-" + strconv.Itoa(i),
				Name:     "job dup test " + strconv.Itoa(i),
				Status:   actions_model.StatusSuccess.String(),
				CanRerun: false,
				Duration: "2m",
				Needs:    []string{"job-103", "job-101", "job-100"},
			})
		}
	}

	fillViewRunResponseCurrentJob(ctx, resp)
	ctx.JSON(http.StatusOK, resp)
}

func fillViewRunResponseCurrentJob(ctx *context.Context, resp *actions.ViewResponse) {
	jobID := ctx.PathParamInt64("job")
	if jobID == 0 {
		return
	}

	for _, job := range resp.State.Run.Jobs {
		if job.ID == jobID {
			resp.State.CurrentJob.Title = job.Name
			resp.State.CurrentJob.Detail = job.Status
			break
		}
	}

	req := web.GetForm(ctx).(*actions.ViewRequest)
	var mockLogOptions []generateMockStepsLogOptions
	resp.State.CurrentJob.Steps = append(resp.State.CurrentJob.Steps, &actions.ViewJobStep{
		Summary:  "step 0 (mock slow)",
		Duration: time.Hour.String(),
		Status:   actions_model.StatusRunning.String(),
	})
	mockLogOptions = append(mockLogOptions, generateMockStepsLogOptions{mockCountFirst: 30, mockCountGeneral: 1, groupRepeat: 3})

	resp.State.CurrentJob.Steps = append(resp.State.CurrentJob.Steps, &actions.ViewJobStep{
		Summary:  "step 1 (mock fast)",
		Duration: time.Hour.String(),
		Status:   actions_model.StatusRunning.String(),
	})
	mockLogOptions = append(mockLogOptions, generateMockStepsLogOptions{mockCountFirst: 30, mockCountGeneral: 3, groupRepeat: 20})

	resp.State.CurrentJob.Steps = append(resp.State.CurrentJob.Steps, &actions.ViewJobStep{
		Summary:  "step 2 (mock error)",
		Duration: time.Hour.String(),
		Status:   actions_model.StatusRunning.String(),
	})
	mockLogOptions = append(mockLogOptions, generateMockStepsLogOptions{mockCountFirst: 30, mockCountGeneral: 3, groupRepeat: 3})

	if len(req.LogCursors) == 0 {
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
		resp.Logs.StepsLog = append(resp.Logs.StepsLog, generateMockStepsLog(logCur, mockLogOptions[logCur.Step])...)
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
}

func MockActionsArtifactDownload(ctx *context.Context) {
	artifactName := ctx.PathParam("artifact_name")
	files, ok := mockActionsArtifactFiles[artifactName]
	if !ok {
		ctx.NotFound(nil)
		return
	}

	ctx.Resp.Header().Set("Content-Disposition", httplib.EncodeContentDispositionAttachment(artifactName+".zip"))
	writer := zip.NewWriter(ctx.Resp)
	defer writer.Close()
	for _, file := range files {
		w, err := writer.Create(file.Path)
		if err != nil {
			ctx.ServerError("writer.Create", err)
			return
		}
		if _, err := io.WriteString(w, file.Content); err != nil {
			ctx.ServerError("io.WriteString", err)
			return
		}
	}
}

func MockActionsArtifactPreview(ctx *context.Context) {
	runID := ctx.PathParamInt64("run")
	artifactName := ctx.PathParam("artifact_name")
	files, ok := mockActionsArtifactFiles[artifactName]
	if !ok {
		ctx.NotFound(nil)
		return
	}

	selectedPath := actions.ChoosePreviewPath(mockArtifactFilePaths(files), actions.GetRequestedPreviewPath(ctx))
	previewFiles := make([]actions.ArtifactPreviewFile, 0, len(files))
	for _, file := range files {
		previewFiles = append(previewFiles, actions.ArtifactPreviewFile{
			Path:     file.Path,
			Selected: file.Path == selectedPath,
		})
	}

	runURL := fmt.Sprintf("%s/devtest/repo-action-view/runs/%d", setting.AppSubURL, runID)
	previewURL := runURL + "/artifacts/" + url.PathEscape(artifactName) + "/preview"

	ctx.Data["ArtifactName"] = artifactName
	ctx.Data["PreviewFiles"] = previewFiles
	ctx.Data["RunURL"] = runURL
	ctx.Data["PreviewURL"] = previewURL
	ctx.Data["PreviewRawURL"] = previewURL + "/raw"
	ctx.Data["DownloadURL"] = runURL + "/artifacts/" + url.PathEscape(artifactName)
	ctx.Data["SelectedPath"] = selectedPath
	ctx.HTML(http.StatusOK, "devtest/repo-action-artifact-preview")
}

func MockActionsArtifactPreviewRaw(ctx *context.Context) {
	artifactName := ctx.PathParam("artifact_name")
	files, ok := mockActionsArtifactFiles[artifactName]
	if !ok {
		actions.WritePreviewRawError(ctx, http.StatusNotFound, "artifact not found")
		return
	}

	selectedPath := actions.ChoosePreviewPath(mockArtifactFilePaths(files), actions.GetRequestedPreviewPath(ctx))
	if selectedPath == "" {
		actions.WritePreviewRawError(ctx, http.StatusNotFound, "artifact file not found")
		return
	}

	var selectedFile *mockArtifactFile
	for i := range files {
		if files[i].Path == selectedPath {
			selectedFile = &files[i]
			break
		}
	}
	if selectedFile == nil {
		actions.WritePreviewRawError(ctx, http.StatusNotFound, "artifact file not found")
		return
	}

	contentType := "text/plain; charset=utf-8"
	if path.Ext(selectedFile.Path) == ".html" {
		contentType = "text/html"
	}
	size := int64(len(selectedFile.Content))
	ctx.ServeContent(strings.NewReader(selectedFile.Content), context.ServeHeaderOptions{
		Filename:      selectedFile.Path,
		ContentLength: &size,
		ContentType:   contentType,
	})
}
