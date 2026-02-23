// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"strconv"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/log"
)

// GetWorkflowSummary returns a human-readable summary of the workflow
func GetWorkflowSummary(ctx context.Context, wf *project_model.Workflow) string {
	filters := wf.WorkflowFilters
	if len(filters) == 0 {
		return ""
	}

	var summary strings.Builder
	labelIDs := make([]int64, 0)
	for _, filter := range filters {
		switch filter.Type {
		case project_model.WorkflowFilterTypeIssueType:
			switch filter.Value {
			case "issue":
				if summary.Len() > 0 {
					summary.WriteString(" ")
				}
				summary.WriteString("(Issues only)")
			case "pull_request":
				if summary.Len() > 0 {
					summary.WriteString(" ")
				}
				summary.WriteString("(Pull requests only)")
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
			if summary.Len() > 0 {
				summary.WriteString(" ")
			}
			summary.WriteString("(Source: " + col.Title + ")")
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
			if summary.Len() > 0 {
				summary.WriteString(" ")
			}
			summary.WriteString("(Target: " + col.Title + ")")
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
			if summary.Len() > 0 {
				summary.WriteString(" ")
			}
			summary.WriteString("(Labels: ")
			for i, label := range labels {
				summary.WriteString(label.Name)
				if i < len(labels)-1 {
					summary.WriteString(", ")
				}
			}
			summary.WriteString(")")
		}
	}
	return summary.String()
}
