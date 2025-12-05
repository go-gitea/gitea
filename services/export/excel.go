// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package export

import (
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"

	"github.com/xuri/excelize/v2"
)

func IssuesToExcel(ctx *context.Context, issues issues_model.IssueList) *excelize.File {
	f := excelize.NewFile()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		log.Error("cannot open stream writer for Sheet1: %v", err)
		return f
	}
	// print headers
	cell, err := excelize.CoordinatesToCellName(1, 1)
	if err != nil {
		log.Error("cannot get first cell: %v", err)
		return f
	}
	err = sw.SetRow(cell, []any{
		excelize.Cell{Value: "ID"},
		excelize.Cell{Value: "Title"},
		excelize.Cell{Value: "Status"},
		excelize.Cell{Value: "Assignee(s)"},
		excelize.Cell{Value: "Label(s)"},
		excelize.Cell{Value: "Created At"},
	})
	if err != nil {
		log.Error("cannot SetRow for header: %v", err)
		return f
	}

	// built-in format ID 22 ("m/d/yy h:mm")
	datetimeStyleID, err := f.NewStyle(&excelize.Style{NumFmt: 22})
	if err != nil {
		log.Error("cannot set new style NumFmt: %v", err)
		return f
	}

	for i, issue := range issues {
		assignees := ""
		if err := issue.LoadAssignees(ctx); err == nil {
			if len(issue.Assignees) > 0 {
				for _, assignee := range issue.Assignees {
					if assignees != "" {
						assignees += ", "
					}
					if assignee.FullName != "" {
						assignees += assignee.FullName
					} else {
						assignees += assignee.Name
					}
				}
			}
		}

		labels := ""
		if err := issue.LoadLabels(ctx); err == nil {
			if len(issue.Labels) > 0 {
				for _, label := range issue.Labels {
					if labels != "" {
						labels += ", "
					}
					labels += label.Name
				}
			}
		}

		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		err = sw.SetRow(cell, []any{
			excelize.Cell{Value: issue.Index},
			excelize.Cell{Value: issue.Title},
			excelize.Cell{Value: issue.State()},
			excelize.Cell{Value: assignees},
			excelize.Cell{Value: labels},
			excelize.Cell{StyleID: datetimeStyleID, Value: issue.CreatedUnix.AsTime()},
		})
		if err != nil {
			log.Error("cannot SetRow: %v", err)
			return f
		}
	}

	sw.Flush()

	return f
}
