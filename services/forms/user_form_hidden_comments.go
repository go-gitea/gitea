// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forms

import (
	"math/big"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

type hiddenCommentTypeGroupsType map[string][]models.CommentType

// hiddenCommentTypeGroups maps the group names to comment types, these group names comes from the Web UI (appearance.tmpl)
var hiddenCommentTypeGroups = hiddenCommentTypeGroupsType{
	"reference": {
		/*3*/ models.CommentTypeIssueRef,
		/*4*/ models.CommentTypeCommitRef,
		/*5*/ models.CommentTypeCommentRef,
		/*6*/ models.CommentTypePullRef,
	},
	"label": {
		/*7*/ models.CommentTypeLabel,
	},
	"milestone": {
		/*8*/ models.CommentTypeMilestone,
	},
	"assignee": {
		/*9*/ models.CommentTypeAssignees,
	},
	"title": {
		/*10*/ models.CommentTypeChangeTitle,
	},
	"branch": {
		/*11*/ models.CommentTypeDeleteBranch,
		/*25*/ models.CommentTypeChangeTargetBranch,
	},
	"time_tracking": {
		/*12*/ models.CommentTypeStartTracking,
		/*13*/ models.CommentTypeStopTracking,
		/*14*/ models.CommentTypeAddTimeManual,
		/*15*/ models.CommentTypeCancelTracking,
		/*26*/ models.CommentTypeDeleteTimeManual,
	},
	"deadline": {
		/*16*/ models.CommentTypeAddedDeadline,
		/*17*/ models.CommentTypeModifiedDeadline,
		/*18*/ models.CommentTypeRemovedDeadline,
	},
	"dependency": {
		/*19*/ models.CommentTypeAddDependency,
		/*20*/ models.CommentTypeRemoveDependency,
	},
	"lock": {
		/*23*/ models.CommentTypeLock,
		/*24*/ models.CommentTypeUnlock,
	},
	"review_request": {
		/*27*/ models.CommentTypeReviewRequest,
	},
	"pull_request_push": {
		/*29*/ models.CommentTypePullRequestPush,
	},
	"project": {
		/*30*/ models.CommentTypeProject,
		/*31*/ models.CommentTypeProjectBoard,
	},
	"issue_ref": {
		/*33*/ models.CommentTypeChangeIssueRef,
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
		return
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
