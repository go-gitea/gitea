// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package export

import (
	"fmt"
	"github.com/xuri/excelize/v2"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/services/context"
)

func IssuesToExcel(ctx *context.Context, issues issues_model.IssueList) *excelize.File {
	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	headers := []string{"ID", "Title", "Status", "Assignee(s)", "Label(s)", "Created At"}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, h)
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

		f.SetCellValue(sheet, fmt.Sprintf("A%d", i+2), issue.Index)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", i+2), issue.Title)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", i+2), issue.State())
		f.SetCellValue(sheet, fmt.Sprintf("D%d", i+2), assignees)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", i+2), labels)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", i+2), issue.CreatedUnix.AsTime()) // .Format("2006-01-02"))
	}
	return f
}