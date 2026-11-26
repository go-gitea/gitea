// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import issues_model "gitea.dev/models/issues"

// ApprovalCountsFunc returns the template func that counts an issue's official "approve", "reject" or "waiting" reviews
func ApprovalCountsFunc(approvalCounts map[int64][]*issues_model.ReviewCount) func(issueID int64, typ string) int64 {
	return func(issueID int64, typ string) int64 {
		reviewTyp := issues_model.ReviewTypeApprove
		switch typ {
		case "reject":
			reviewTyp = issues_model.ReviewTypeReject
		case "waiting":
			reviewTyp = issues_model.ReviewTypeRequest
		}
		for _, count := range approvalCounts[issueID] {
			if count.Type == reviewTyp {
				return count.Count
			}
		}
		return 0
	}
}
