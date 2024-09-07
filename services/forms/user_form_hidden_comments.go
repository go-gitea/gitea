// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"math/big"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
)

type hiddenCommentTypeGroupsType map[string][]issues_model.CommentType

// hiddenCommentTypeGroups maps the group names to comment types, these group names comes from the Web UI (appearance.tmpl)
var hiddenCommentTypeGroups = hiddenCommentTypeGroupsType{
	"reference": {
		/*3*/ issues_model.CommentTypeIssueRef,
		/*4*/ issues_model.CommentTypeCommitRef,
		/*5*/ issues_model.CommentTypeCommentRef,
		/*6*/ issues_model.CommentTypePullRef,
	},
	"label": {
		/*7*/ issues_model.CommentTypeLabel,
	},
	"milestone": {
		/*8*/ issues_model.CommentTypeMilestone,
	},
	"assignee": {
		/*9*/ issues_model.CommentTypeAssignees,
	},
	"title": {
		/*10*/ issues_model.CommentTypeChangeTitle,
	},
	"branch": {
		/*11*/ issues_model.CommentTypeDeleteBranch,
		/*25*/ issues_model.CommentTypeChangeTargetBranch,
	},
	"time_tracking": {
		/*12*/ issues_model.CommentTypeStartTracking,
		/*13*/ issues_model.CommentTypeStopTracking,
		/*14*/ issues_model.CommentTypeAddTimeManual,
		/*15*/ issues_model.CommentTypeCancelTracking,
		/*26*/ issues_model.CommentTypeDeleteTimeManual,
	},
	"deadline": {
		/*16*/ issues_model.CommentTypeAddedDeadline,
		/*17*/ issues_model.CommentTypeModifiedDeadline,
		/*18*/ issues_model.CommentTypeRemovedDeadline,
	},
	"dependency": {
		/*19*/ issues_model.CommentTypeAddDependency,
		/*20*/ issues_model.CommentTypeRemoveDependency,
	},
	"lock": {
		/*23*/ issues_model.CommentTypeLock,
		/*24*/ issues_model.CommentTypeUnlock,
	},
	"review_request": {
		/*27*/ issues_model.CommentTypeReviewRequest,
	},
	"pull_request_push": {
		/*29*/ issues_model.CommentTypePullRequestPush,
	},
	"project": {
		/*30*/ issues_model.CommentTypeProject,
		/*31*/ issues_model.CommentTypeProjectColumn,
	},
	"issue_ref": {
		/*33*/ issues_model.CommentTypeChangeIssueRef,
	},
}

// UserHiddenCommentTypesFromRequest parse the form to hidden comment types bitset
func UserHiddenCommentTypesFromRequest(ctx *context.Context) *big.Int {
	bitset := new(big.Int)
	for group, commentTypes := range hiddenCommentTypeGroups {
		if ctx.FormBool(group) {
			for _, commentType := range commentTypes {
				bitset = bitset.SetBit(bitset, int(commentType), 1)
			}
		}
	}
	return bitset
}

// IsUserHiddenCommentTypeGroupChecked check whether a hidden comment type group is "enabled" (checked on UI)
func IsUserHiddenCommentTypeGroupChecked(group string, hiddenCommentTypes *big.Int) (ret bool) {
	commentTypes, ok := hiddenCommentTypeGroups[group]
	if !ok {
		log.Critical("the group map for hidden comment types is out of sync, unknown group: %v", group)
		return false
	}
	if hiddenCommentTypes == nil {
		return false
	}
	for _, commentType := range commentTypes {
		if hiddenCommentTypes.Bit(int(commentType)) == 1 {
			return true
		}
	}
	return false
}
