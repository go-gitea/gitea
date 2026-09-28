// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"cmp"
	"slices"
	"strings"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/modules/container"
)

// LabelSwimlane contains a label's visible issues, arranged by project column.
type LabelSwimlane struct {
	LabelID   int64
	Label     *issues_model.Label
	Issues    map[int64]issues_model.IssueList
	NumIssues int
}

// BuildLabelSwimlanes selects label rows from visible issues and counts unique issues per column.
func BuildLabelSwimlanes(columns []*project_model.Column, issuesMap map[int64]issues_model.IssueList, labelIDs []int64) ([]*LabelSwimlane, map[int64]int64) {
	included := make(container.Set[int64])
	excluded := make(container.Set[int64])
	for _, labelID := range labelIDs {
		if labelID < 0 {
			excluded.Add(-labelID)
		} else {
			included.Add(labelID)
		}
	}
	lanes := make(map[int64]*LabelSwimlane)
	counts := make(map[int64]int64)
	add := func(label *issues_model.Label, columnID int64, issue *issues_model.Issue) bool {
		var labelID int64
		if label != nil {
			labelID = label.ID
		}
		if excluded.Contains(labelID) || (len(included) > 0 && !included.Contains(labelID)) {
			return false
		}
		lane := lanes[labelID]
		if lane == nil {
			lane = &LabelSwimlane{LabelID: labelID, Label: label, Issues: make(map[int64]issues_model.IssueList)}
			lanes[labelID] = lane
		}
		lane.Issues[columnID] = append(lane.Issues[columnID], issue)
		lane.NumIssues++
		return true
	}
	for _, column := range columns {
		for _, issue := range issuesMap[column.ID] {
			visible := false
			if len(issue.Labels) == 0 {
				visible = add(nil, column.ID, issue)
			}
			for _, label := range issue.Labels {
				if add(label, column.ID, issue) {
					visible = true
				}
			}
			if visible {
				counts[column.ID]++
			}
		}
	}
	result := make([]*LabelSwimlane, 0, len(lanes))
	for id, lane := range lanes {
		if id != 0 {
			result = append(result, lane)
		}
	}
	slices.SortFunc(result, func(a, b *LabelSwimlane) int {
		if order := strings.Compare(strings.ToLower(a.Label.Name), strings.ToLower(b.Label.Name)); order != 0 {
			return order
		}
		return cmp.Compare(a.LabelID, b.LabelID)
	})
	if unlabeled := lanes[0]; unlabeled != nil {
		result = append(result, unlabeled)
	}
	return result, counts
}
