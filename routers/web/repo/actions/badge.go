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
	Width         string
	LabelWidth    string
	MessageWidth  string
	LabelX        string
	MessageX      string
	LabelLength   string
	MessageLength string
	MessageColor  string
	MessageShadow string
	FontSize      string
}

const (
	DEFAULT_OFFSET    = 9
	DEFAULT_SPACING   = 0
	DEFAULT_FONT_SIZE = 11

	COLOR_BLUE        = "#007ec6"
	COLOR_BRIGHTGREEN = "#4c1"
	COLOR_GREEN       = "#97ca00"
	COLOR_GREY        = "#555"
	COLOR_LIGHTGREY   = "#9f9f9f"
	COLOR_ORANGE      = "#fe7d37"
	COLOR_RED         = "#e05d44"
	COLOR_YELLOW      = "#dfb317"
	COLOR_YELLOWGREEN = "#a4a61d"

	COLOR_SUCCESS       = "#4c1"
	COLOR_IMPORTANT     = "#fe7d37"
	COLOR_CRITICAL      = "#e05d44"
	COLOR_INFORMATIONAL = "#007ec6"
	COLOR_INACTIVE      = "#9f9f9f"
)

var (
	drawer = &font.Drawer{
		Face: basicfont.Face7x13,
	}
)

type badgeInfo struct {
	color  string
	status string
}

var statusBadgeInfoMap = map[actions_model.Status]badgeInfo{
	actions_model.StatusSuccess:   {COLOR_SUCCESS, "success"},
	actions_model.StatusSkipped:   {COLOR_YELLOW, "skipped"},
	actions_model.StatusUnknown:   {COLOR_GREEN, "unknown"},
	actions_model.StatusFailure:   {COLOR_CRITICAL, "failure"},
	actions_model.StatusCancelled: {COLOR_IMPORTANT, "cancelled"},
	actions_model.StatusWaiting:   {COLOR_YELLOW, "waiting"},
	actions_model.StatusRunning:   {COLOR_YELLOW, "running"},
	actions_model.StatusBlocked:   {COLOR_YELLOW, "blocked"},
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

	badgeInfo, ok := statusBadgeInfoMap[run.Status]
	if !ok {
		return Badge{}, fmt.Errorf("unknown status %d", run.Status)
	}

	extension := filepath.Ext(workflowFile)
	workflowName := strings.TrimSuffix(workflowFile, extension)
	return generateBadge(workflowName, badgeInfo.status, badgeInfo.color), nil
}

//utils for badge generation -------------------------------------

// generateBadge generates badge with given template
func generateBadge(label, message, color string) Badge {
	c := parseColor(color)
	gF := float64(DEFAULT_OFFSET)
	lW := float64(drawer.MeasureString(label)>>6) + gF
	mW := float64(drawer.MeasureString(message)>>6) + gF
	fW := lW + mW
	lX := (lW/2 + 1) * 10
	mX := (lW + (mW / 2) - 1) * 10
	lL := (lW - gF) * (10.0 + DEFAULT_SPACING - 0.5)
	mL := (mW - gF) * (10.0 + DEFAULT_SPACING - 0.5)
	fS := DEFAULT_FONT_SIZE * 10

	mC, mS := getMessageColors(c)

	return Badge{
		Label:         label,
		Message:       message,
		Color:         formatColor(c),
		Width:         formatFloat(fW),
		LabelWidth:    formatFloat(lW),
		MessageWidth:  formatFloat(mW),
		LabelX:        formatFloat(lX),
		MessageX:      formatFloat(mX),
		LabelLength:   formatFloat(lL),
		MessageLength: formatFloat(mL),
		MessageColor:  mC,
		MessageShadow: mS,
		FontSize:      strconv.Itoa(fS),
	}
}

// parseColor parses hex color
func parseColor(c string) int64 {
	if strings.HasPrefix(c, "#") {
		c = strings.TrimLeft(c, "#")
	}

	// Shorthand hex color
	if len(c) == 3 {
		c = strings.Repeat(string(c[0]), 2) +
			strings.Repeat(string(c[1]), 2) +
			strings.Repeat(string(c[2]), 2)
	}

	i, _ := strconv.ParseInt(c, 16, 32)

	return i
}

// formatColor formats color
func formatColor(c int64) string {
	k := fmt.Sprintf("%06x", c)

	if k[0] == k[1] && k[2] == k[3] && k[4] == k[5] {
		k = k[0:1] + k[2:3] + k[4:5]
	}

	return "#" + k
}

// formatFloat formats float values
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64)
}

// getMessageColors returns message text and shadow colors based on color of badge
func getMessageColors(c int64) (string, string) {
	if c == 0 || calcLuminance(c) < 0.65 {
		return "#fff", "#010101"
	}

	return "#333", "#ccc"
}

// calcLuminance calculates relative luminance
func calcLuminance(color int64) float64 {
	r := calcLumColor(float64(color>>16&0xFF) / 255)
	g := calcLumColor(float64(color>>8&0xFF) / 255)
	b := calcLumColor(float64(color&0xFF) / 255)

	return 0.2126*r + 0.7152*g + 0.0722*b
}

// calcLumColor calculates luminance for one color
func calcLumColor(c float64) float64 {
	if c <= 0.03928 {
		return c / 12.92
	}

	return math.Pow(((c + 0.055) / 1.055), 2.4)
}
