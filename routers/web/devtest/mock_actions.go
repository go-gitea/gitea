// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"archive/zip"
	"fmt"
	"html/template"
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
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/routers/web/repo/actions"
	"code.gitea.io/gitea/services/context"
)

type mockArtifactFile struct {
	Path    string
	Content string
}

type mockArtifactPreviewTemplateData struct {
	ArtifactName string
	Files        []mockArtifactPreviewTemplateFile
	PreviewURL   string
	PreviewRaw   string
	DownloadURL  string
	SelectedPath string
}

type mockArtifactPreviewTemplateFile struct {
	Path     string
	Selected bool
}

var mockActionsArtifactFiles = map[string][]mockArtifactFile{
	"artifact-b": {
		{
			Path:    "report.txt",
			Content: "artifact-b report",
		},
	},
	"artifact-really-loooooooooooooooooooooooooooooooooooooooooooooooooooooooong": {
		{
			Path: "index.html",
			Content: `<!doctype html>
<html>
  <body>
    <h1>Mock Artifact Preview</h1>
    <p>artifact-really-loooooong</p>
  </body>
</html>`,
		},
		{
			Path:    "logs/output.txt",
			Content: "mock logs",
		},
	},
}

var mockArtifactPreviewTemplate = template.Must(template.New("mock-artifact-preview").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Artifact Preview</title>
  <style>
    body { font-family: sans-serif; margin: 16px; }
    .layout { display: grid; grid-template-columns: 260px 1fr; gap: 16px; }
    .files { border: 1px solid #ddd; border-radius: 6px; padding: 8px; }
    .files a { display: block; padding: 8px; text-decoration: none; color: inherit; border-radius: 4px; }
    .files a.selected { background: #f0f6ff; }
    iframe { width: 100%; min-height: 70vh; border: 1px solid #ddd; border-radius: 6px; }
  </style>
</head>
<body>
  <h2>Preview: {{.ArtifactName}}</h2>
  <p><a href="{{.PreviewURL}}">Reload</a> | <a href="{{.DownloadURL}}">Download ZIP</a></p>
  <div class="layout">
    <div class="files">
      {{range .Files}}
        <a class="{{if .Selected}}selected{{end}}" href="{{$.PreviewURL}}?path={{.Path | urlquery}}">{{.Path}}</a>
      {{end}}
    </div>
    <div>
      {{if .SelectedPath}}
        <iframe src="{{.PreviewRaw}}?path={{.SelectedPath | urlquery}}" referrerpolicy="same-origin"></iframe>
      {{else}}
        <p>No files</p>
      {{end}}
    </div>
  </div>
</body>
</html>`))

func normalizeMockArtifactPath(path string) string {
	path = util.PathJoinRelX(path)
	if path == "." {
		return ""
	}
	return path
}

func getMockArtifactFiles(name string) ([]mockArtifactFile, bool) {
	files, ok := mockActionsArtifactFiles[name]
	return files, ok
}

func chooseMockArtifactPath(files []mockArtifactFile, requestedPath string) string {
	if len(files) == 0 {
		return ""
	}
	for _, file := range files {
		if file.Path == requestedPath {
			return requestedPath
		}
	}
	return files[0].Path
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
	ctx.Data["RunIndex"] = ctx.PathParam("run")
	ctx.Data["JobIndex"] = ctx.PathParam("job")
	ctx.HTML(http.StatusOK, "devtest/repo-action-view")
}

func MockActionsRunsJobs(ctx *context.Context) {
	runID := ctx.PathParamInt64("run")

	req := web.GetForm(ctx).(*actions.ViewRequest)
	resp := &actions.ViewResponse{}
	resp.State.Run.TitleHTML = `mock run title <a href="/">link</a>`
	resp.State.Run.Link = setting.AppSubURL + "/devtest/repo-action-view/runs/" + strconv.FormatInt(runID, 10)
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
	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:   "artifact-very-loooooooooooooooooooooooooooooooooooooooooooooooooooooooong",
		Size:   100 * 1024,
		Status: "expired",
	})
	resp.Artifacts = append(resp.Artifacts, &actions.ArtifactsViewItem{
		Name:   "artifact-really-loooooooooooooooooooooooooooooooooooooooooooooooooooooooong",
		Size:   1024 * 1024,
		Status: "completed",
	})

	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID * 10,
		JobID:    "job-100",
		Name:     "job 100",
		Status:   actions_model.StatusRunning.String(),
		CanRerun: true,
		Duration: "1h",
	})
	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID*10 + 1,
		JobID:    "job-101",
		Name:     "job 101",
		Status:   actions_model.StatusWaiting.String(),
		CanRerun: false,
		Duration: "2h",
		Needs:    []string{"job-100"},
	})
	resp.State.Run.Jobs = append(resp.State.Run.Jobs, &actions.ViewJob{
		ID:       runID*10 + 2,
		JobID:    "job-102",
		Name:     "job 102",
		Status:   actions_model.StatusFailure.String(),
		CanRerun: false,
		Duration: "3h",
		Needs:    []string{"job-100", "job-101"},
	})

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
	ctx.JSON(http.StatusOK, resp)
}

func MockActionsArtifactDownload(ctx *context.Context) {
	artifactName := ctx.PathParam("artifact_name")
	files, ok := getMockArtifactFiles(artifactName)
	if !ok {
		ctx.NotFound(nil)
		return
	}

	ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip; filename*=UTF-8''%s.zip", url.PathEscape(artifactName), artifactName))
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
	files, ok := getMockArtifactFiles(artifactName)
	if !ok {
		ctx.NotFound(nil)
		return
	}

	selectedPath := chooseMockArtifactPath(files, normalizeMockArtifactPath(ctx.Req.URL.Query().Get("path")))
	templateFiles := make([]mockArtifactPreviewTemplateFile, 0, len(files))
	for _, file := range files {
		templateFiles = append(templateFiles, mockArtifactPreviewTemplateFile{
			Path:     file.Path,
			Selected: file.Path == selectedPath,
		})
	}

	previewURL := fmt.Sprintf("%s/devtest/repo-action-view/runs/%d/artifacts/%s/preview", setting.AppSubURL, runID, url.PathEscape(artifactName))
	previewRawURL := previewURL + "/raw"
	downloadURL := fmt.Sprintf("%s/devtest/repo-action-view/runs/%d/artifacts/%s", setting.AppSubURL, runID, url.PathEscape(artifactName))
	data := mockArtifactPreviewTemplateData{
		ArtifactName: artifactName,
		Files:        templateFiles,
		PreviewURL:   previewURL,
		PreviewRaw:   previewRawURL,
		DownloadURL:  downloadURL,
		SelectedPath: selectedPath,
	}

	ctx.Resp.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := mockArtifactPreviewTemplate.Execute(ctx.Resp, data); err != nil {
		ctx.ServerError("mockArtifactPreviewTemplate.Execute", err)
		return
	}
}

func MockActionsArtifactPreviewRaw(ctx *context.Context) {
	artifactName := ctx.PathParam("artifact_name")
	files, ok := getMockArtifactFiles(artifactName)
	if !ok {
		ctx.NotFound(nil)
		return
	}

	selectedPath := chooseMockArtifactPath(files, normalizeMockArtifactPath(ctx.Req.URL.Query().Get("path")))
	if selectedPath == "" {
		ctx.NotFound(nil)
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
		ctx.NotFound(nil)
		return
	}

	if path.Ext(selectedFile.Path) == ".html" {
		ctx.Resp.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		httplib.ServeContentByReader(ctx.Req, ctx.Resp, int64(len(selectedFile.Content)), strings.NewReader(selectedFile.Content), &httplib.ServeHeaderOptions{
			Filename:    selectedFile.Path,
			ContentType: "text/html",
		})
		return
	}
	common.ServeContentByReader(ctx.Base, selectedFile.Path, int64(len(selectedFile.Content)), strings.NewReader(selectedFile.Content))
}
