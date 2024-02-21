// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

type Label struct {
	text  string
	width float64
}

func (l Label) Label() string {
	return l.text
}

func (l Label) Width() int {
	return int(l.width)
}

func (l Label) TextLength() int {
	return int(l.width * 9.5) // 95% of the width
}

func (l Label) X() int {
	return int((l.width/2 + 1) * 10) // scale 10 times to be more accurate
}

type Message struct {
	text  string
	width float64
	x     int
}

func (m Message) Message() string {
	return m.text
}

func (m Message) Width() int {
	return int(m.width)
}

func (m Message) X() int {
	return m.x
}

func (m Message) TextLength() int {
	return int(m.width * 9.5)
}

type Badge struct {
	Color    string
	FontSize int
	Label    Label
	Message  Message
}

func (b Badge) Width() int {
	return b.Label.Width() + b.Message.Width()
}

const (
	defaultOffset   = 9
	defaultFontSize = 11
)

var drawer = &font.Drawer{
	Face: basicfont.Face7x13,
}

var statusColorMap = map[actions_model.Status]string{
	actions_model.StatusSuccess:   "#4c1",    // Green
	actions_model.StatusSkipped:   "#dfb317", // Yellow
	actions_model.StatusUnknown:   "#97ca00", // Light Green
	actions_model.StatusFailure:   "#e05d44", // Red
	actions_model.StatusCancelled: "#fe7d37", // Orange
	actions_model.StatusWaiting:   "#dfb317", // Yellow
	actions_model.StatusRunning:   "#dfb317", // Yellow
	actions_model.StatusBlocked:   "#dfb317", // Yellow
}

func GetWorkflowBadge(ctx *context.Context) {
	workflowFile := ctx.Params("workflow_name")
	branch := ctx.Req.URL.Query().Get("branch")
	if branch == "" {
		branch = ctx.Repo.Repository.DefaultBranch
	}
	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	event := ctx.Req.URL.Query().Get("event")
	badge, err := getWorkflowBadge(ctx, workflowFile, branchRef, event)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound("Not found", fmt.Errorf("%s not found", workflowFile))
			return
		}
		ctx.ServerError("GetWorkflowBadge", err)
		return
	}

	ctx.Data["Badge"] = badge
	ctx.RespHeader().Set("Content-Type", "image/svg+xml")
	ctx.HTML(http.StatusOK, "shared/actions/runner_badge")
}

func getWorkflowBadge(ctx *context.Context, workflowFile, branchName, event string) (Badge, error) {
	run, err := actions_model.GetWorkflowLatestRun(ctx, ctx.Repo.Repository.ID, workflowFile, branchName, event)
	if err != nil {
		return Badge{}, err
	}

	color, ok := statusColorMap[run.Status]
	if !ok {
		return Badge{}, fmt.Errorf("unknown status %d", run.Status)
	}

	extension := filepath.Ext(workflowFile)
	workflowName := strings.TrimSuffix(workflowFile, extension)
	return generateBadge(workflowName, run.Status.String(), color), nil
}

// generateBadge generates badge with given template
func generateBadge(label, message, color string) Badge {
	lw := float64(drawer.MeasureString(label)>>6) + float64(defaultOffset)
	mw := float64(drawer.MeasureString(message)>>6) + float64(defaultOffset)
	x := int((lw + (mw / 2) - 1) * 10)
	return Badge{
		Label: Label{
			text:  label,
			width: lw,
		},
		Message: Message{
			text:  message,
			width: mw,
			x:     x,
		},
		FontSize: defaultFontSize * 10,
	}
}
