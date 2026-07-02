// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"strconv"
	"strings"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/modules/log"
	"gitea.dev/modules/translation"
)

// GetWorkflowSummary returns a human-readable summary of the workflow
func GetWorkflowSummary(ctx context.Context, wf *project_model.Workflow) string {
	filters := wf.WorkflowFilters
	if len(filters) == 0 {
		return ""
	}

	locale, ok := ctx.Value(translation.ContextKey).(translation.Locale)
	if !ok {
		locale = translation.NewLocale("en-US")
	}

	appendSummaryPart := func(summary *strings.Builder, text string) {
		if text == "" {
			return
		}
		if summary.Len() > 0 {
			summary.WriteString(" ")
		}
		summary.WriteString("(")
		summary.WriteString(text)
		summary.WriteString(")")
	}

	var summary strings.Builder
	labelIDs := make([]int64, 0)
	for _, filter := range filters {
		switch filter.Type {
		case project_model.WorkflowFilterTypeIssueType:
			switch filter.Value {
			case "issue":
				appendSummaryPart(&summary, locale.TrString("projects.workflows.issues_only"))
			case "pull_request":
				appendSummaryPart(&summary, locale.TrString("projects.workflows.pull_requests_only"))
			}
		case project_model.WorkflowFilterTypeSourceColumn:
			columnID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if columnID <= 0 {
				continue
			}
			col, err := project_model.GetColumn(ctx, columnID)
			if err != nil {
				log.Error("GetColumn: %v", err)
				continue
			}
			appendSummaryPart(&summary, locale.TrString("projects.workflows.summary.source", col.Title))
		case project_model.WorkflowFilterTypeTargetColumn:
			columnID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if columnID <= 0 {
				continue
			}
			col, err := project_model.GetColumn(ctx, columnID)
			if err != nil {
				log.Error("GetColumn: %v", err)
				continue
			}
			appendSummaryPart(&summary, locale.TrString("projects.workflows.summary.target", col.Title))
		case project_model.WorkflowFilterTypeLabels:
			labelID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if labelID > 0 {
				labelIDs = append(labelIDs, labelID)
			}
		}
	}
	if len(labelIDs) > 0 {
		labels, err := issues_model.GetLabelsByIDs(ctx, labelIDs)
		if err != nil {
			log.Error("GetLabelsByIDs: %v", err)
		} else {
			labelNames := make([]string, 0, len(labels))
			for _, label := range labels {
				labelNames = append(labelNames, label.Name)
			}
			appendSummaryPart(&summary, locale.TrString("projects.workflows.summary.labels", strings.Join(labelNames, ", ")))
		}
	}
	return summary.String()
}
