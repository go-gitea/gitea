// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"errors"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/context"
)

// AddAndOrRemoveLabelFromIssue updates issue label according to board LabelID
func AddAndOrRemoveLabelFromIssue(ctx *context.Context, currentIssueId int64, fromColumnLabelID int64, board *project_model.Board) error {
	issue, err := issues_model.GetIssueByID(ctx, currentIssueId)
	if err != nil {
		return errors.New("failed getting issue")
	}
	var addedLabel *issues_model.Label
	if board.LabelID != 0 {
		addedLabel, err = issues_model.GetLabelByID(ctx, board.LabelID)
		if err != nil {
			return errors.New("failed getting add label")
		}
	}
	var removedLabel *issues_model.Label
	if fromColumnLabelID != 0 {

		removedLabel, err = issues_model.GetLabelByID(ctx, fromColumnLabelID)
		if err != nil {
			return errors.New("failed getting remove label")
		}
	}

	// Delete old label from current issue
	if fromColumnLabelID != 0 {
		if err := RemoveLabel(ctx, issue, ctx.Doer, removedLabel); err != nil {
			return err
		}
	}

	// Add New Label to current issue
	if board.LabelID != 0 {
		if err := AddLabel(ctx, issue, ctx.Doer, addedLabel); err != nil {
			return err
		}
	}

	return nil
}
