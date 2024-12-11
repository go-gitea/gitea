// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/services/context"
)

// PrepareFilterIssueLabels reads the "labels" query parameter, sets `ctx.Data["Labels"]` and `ctx.Data["SelectLabels"]`
func PrepareFilterIssueLabels(ctx *context.Context, repoID int64, owner *user_model.User) (labelIDs []int64) {
	// 1,-2 means including label 1 and excluding label 2
	// 0 means issues with no label
	// blank means labels will not be filtered for issues
	selectLabels := ctx.FormString("labels")
	if selectLabels != "" {
		var err error
		labelIDs, err = base.StringsToInt64s(strings.Split(selectLabels, ","))
		if err != nil {
			ctx.Flash.Error(ctx.Tr("invalid_data", selectLabels), true)
		}
	}

	var allLabels []*issues_model.Label
	if repoID != 0 {
		repoLabels, err := issues_model.GetLabelsByRepoID(ctx, repoID, "", db.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByRepoID", err)
			return nil
		}
		allLabels = append(allLabels, repoLabels...)
	}

	if owner != nil && owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, owner.ID, "", db.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByOrgID", err)
			return nil
		}
		allLabels = append(allLabels, orgLabels...)
	}

	// Get the exclusive scope for every label ID
	labelExclusiveScopes := make([]string, 0, len(labelIDs))
	for _, labelID := range labelIDs {
		foundExclusiveScope := false
		for _, label := range allLabels {
			if label.ID == labelID || label.ID == -labelID {
				labelExclusiveScopes = append(labelExclusiveScopes, label.ExclusiveScope())
				foundExclusiveScope = true
				break
			}
		}
		if !foundExclusiveScope {
			labelExclusiveScopes = append(labelExclusiveScopes, "")
		}
	}

	for _, l := range allLabels {
		l.LoadSelectedLabelsAfterClick(labelIDs, labelExclusiveScopes)
	}
	ctx.Data["Labels"] = allLabels
	ctx.Data["SelectLabels"] = selectLabels
	return labelIDs
}
