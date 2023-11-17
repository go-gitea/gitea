// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

type Badge struct {
	Label         string
	Message       string
	Color         string
	Width         int
	LabelWidth    int
	MessageWidth  int
	LabelX        int
	MessageX      int
	LabelLength   int
	MessageLength int
	MessageColor  string
	MessageShadow string
	FontSize      int
}

const (
	DEFAULT_OFFSET    = 9
	DEFAULT_SPACING   = 0
	DEFAULT_FONT_SIZE = 11
)

var (
	drawer = &font.Drawer{
		Face: basicfont.Face7x13,
	}
)

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

func GetDefaultBranchWorkflowBadge(ctx *context.Context) {
	workflowFile := ctx.Params("workflow_name")
	branchName := git.RefNameFromBranch(ctx.Repo.Repository.DefaultBranch).String()
	badge, err := getWorkflowBadge(ctx, workflowFile, branchName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound("Not found", fmt.Errorf("%s not found", workflowFile))
			return
		}
		ctx.ServerError("GetWorkflowBadge", err)
		return
	}
	ctx.Data["Badge"] = badge
	ctx.HTML(http.StatusOK, "shared/actions/runner_badge")
}
func GetWorkflowBadge(ctx *context.Context) {
	workflowFile := ctx.Params("workflow_name")
	branchName := ctx.Params("branch_name")
	badge, err := getWorkflowBadge(ctx, workflowFile, branchName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound("Not found", fmt.Errorf("%s not found", workflowFile))
			return
		}
		ctx.ServerError("GetWorkflowBadge", err)
		return
	}
	ctx.Data["Badge"] = badge
	ctx.HTML(http.StatusOK, "shared/actions/runner_badge")
}

func getWorkflowBadge(ctx *context.Context, workflowFile string, branchName string) (Badge, error) {
	run, err := actions_model.GetRepoBranchLastRun(ctx, ctx.Repo.Repository.ID, branchName, workflowFile)
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

//utils for badge generation -------------------------------------

// generateBadge generates badge with given template
func generateBadge(label, message, color string) Badge {
	gF := float64(DEFAULT_OFFSET)
	lW := float64(drawer.MeasureString(label)>>6) + gF
	mW := float64(drawer.MeasureString(message)>>6) + gF
	fW := lW + mW
	lX := (lW/2 + 1) * 10
	mX := (lW + (mW / 2) - 1) * 10
	lL := (lW - gF) * (10.0 + DEFAULT_SPACING - 0.5)
	mL := (mW - gF) * (10.0 + DEFAULT_SPACING - 0.5)
	fS := DEFAULT_FONT_SIZE * 10

	mC, mS := determineMessageColorsFromHex(color)

	return Badge{
		Label:         label,
		Message:       message,
		Color:         color,
		Width:         int(fW),
		LabelWidth:    int(lW),
		MessageWidth:  int(mW),
		LabelX:        int(lX),
		MessageX:      int(mX),
		LabelLength:   int(lL),
		MessageLength: int(mL),
		MessageColor:  mC,
		MessageShadow: mS,
		FontSize:      fS,
	}
}

// determineMessageColorsFromHex takes a hex color string and returns text and shadow colors
func determineMessageColorsFromHex(hexColor string) (string, string) {
	hexColor = strings.TrimPrefix(hexColor, "#")
	// Check for shorthand hex color
	if len(hexColor) == 3 {
		hexColor = strings.Repeat(string(hexColor[0]), 2) +
			strings.Repeat(string(hexColor[1]), 2) +
			strings.Repeat(string(hexColor[2]), 2)
	}

	badgeColor, _ := strconv.ParseInt(hexColor, 16, 32)

	if badgeColor == 0 || computeLuminance(badgeColor) < 0.65 {
		return "#fff", "#010101"
	}

	return "#333", "#ccc"
}

// computeLuminance calculates the relative luminance of a color
func computeLuminance(inputColor int64) float64 {
	r := singleColorLuminance(float64(inputColor>>16&0xFF) / 255)
	g := singleColorLuminance(float64(inputColor>>8&0xFF) / 255)
	b := singleColorLuminance(float64(inputColor&0xFF) / 255)

	return 0.2126*r + 0.7152*g + 0.0722*b
}

// singleColorLuminance calculates luminance for a single color
func singleColorLuminance(colorValue float64) float64 {
	if colorValue <= 0.03928 {
		return colorValue / 12.92
	}

	return math.Pow((colorValue+0.055)/1.055, 2.4)
}
