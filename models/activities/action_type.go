// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import "strconv"

// ActionType represents the type of an action.
type ActionType int

// Possible action types.
const (
	ActionCreateRepo                ActionType = iota + 1 // 1
	ActionRenameRepo                                      // 2
	ActionStarRepo                                        // 3
	ActionWatchRepo                                       // 4
	ActionCommitRepo                                      // 5
	ActionCreateIssue                                     // 6
	ActionCreatePullRequest                               // 7
	ActionTransferRepo                                    // 8
	ActionPushTag                                         // 9
	ActionCommentIssue                                    // 10
	ActionMergePullRequest                                // 11
	ActionCloseIssue                                      // 12
	ActionReopenIssue                                     // 13
	ActionClosePullRequest                                // 14
	ActionReopenPullRequest                               // 15
	ActionDeleteTag                                       // 16
	ActionDeleteBranch                                    // 17
	ActionMirrorSyncPush                                  // 18
	ActionMirrorSyncCreate                                // 19
	ActionMirrorSyncDelete                                // 20
	ActionApprovePullRequest                              // 21
	ActionRejectPullRequest                               // 22
	ActionCommentPull                                     // 23
	ActionPublishRelease                                  // 24
	ActionPullReviewDismissed                             // 25
	ActionPullRequestReadyForReview                       // 26
	ActionAutoMergePullRequest                            // 27
)

func (at ActionType) IsActionCreateRepo() bool {
	return at == ActionCreateRepo
}

func (at ActionType) IsActionRenameRepo() bool {
	return at == ActionRenameRepo
}

func (at ActionType) IsActionStarRepo() bool {
	return at == ActionStarRepo
}

func (at ActionType) IsActionWatchRepo() bool {
	return at == ActionWatchRepo
}

func (at ActionType) IsActionCommitRepo() bool {
	return at == ActionCommitRepo
}

func (at ActionType) IsActionCreateIssue() bool {
	return at == ActionCreateIssue
}

func (at ActionType) IsActionCreatePullRequest() bool {
	return at == ActionCreatePullRequest
}

func (at ActionType) IsActionTransferRepo() bool {
	return at == ActionTransferRepo
}

func (at ActionType) IsActionPushTag() bool {
	return at == ActionPushTag
}

func (at ActionType) IsActionCommentIssue() bool {
	return at == ActionCommentIssue
}

func (at ActionType) IsActionMergePullRequest() bool {
	return at == ActionMergePullRequest
}

func (at ActionType) IsActionCloseIssue() bool {
	return at == ActionCloseIssue
}

func (at ActionType) IsActionReopenIssue() bool {
	return at == ActionReopenIssue
}

func (at ActionType) IsActionClosePullRequest() bool {
	return at == ActionClosePullRequest
}

func (at ActionType) IsActionReopenPullRequest() bool {
	return at == ActionReopenPullRequest
}

func (at ActionType) IsActionDeleteTag() bool {
	return at == ActionDeleteTag
}

func (at ActionType) IsActionDeleteBranch() bool {
	return at == ActionDeleteBranch
}

func (at ActionType) IsActionMirrorSyncPush() bool {
	return at == ActionMirrorSyncPush
}

func (at ActionType) IsActionMirrorSyncCreate() bool {
	return at == ActionMirrorSyncCreate
}

func (at ActionType) IsActionMirrorSyncDelete() bool {
	return at == ActionMirrorSyncDelete
}

func (at ActionType) IsActionApprovePullRequest() bool {
	return at == ActionApprovePullRequest
}

func (at ActionType) IsActionRejectPullRequest() bool {
	return at == ActionRejectPullRequest
}

func (at ActionType) IsActionCommentPull() bool {
	return at == ActionCommentPull
}

func (at ActionType) IsActionPublishRelease() bool {
	return at == ActionPublishRelease
}

func (at ActionType) IsActionPullReviewDismissed() bool {
	return at == ActionPullReviewDismissed
}

func (at ActionType) IsActionPullRequestReadyForReview() bool {
	return at == ActionPullRequestReadyForReview
}

func (at ActionType) IsActionAutoMergePullRequest() bool {
	return at == ActionAutoMergePullRequest
}

func (at ActionType) String() string {
	switch at {
	case ActionCreateRepo:
		return "create_repo"
	case ActionRenameRepo:
		return "rename_repo"
	case ActionStarRepo:
		return "star_repo"
	case ActionWatchRepo:
		return "watch_repo"
	case ActionCommitRepo:
		return "commit_repo"
	case ActionCreateIssue:
		return "create_issue"
	case ActionCreatePullRequest:
		return "create_pull_request"
	case ActionTransferRepo:
		return "transfer_repo"
	case ActionPushTag:
		return "push_tag"
	case ActionCommentIssue:
		return "comment_issue"
	case ActionMergePullRequest:
		return "merge_pull_request"
	case ActionCloseIssue:
		return "close_issue"
	case ActionReopenIssue:
		return "reopen_issue"
	case ActionClosePullRequest:
		return "close_pull_request"
	case ActionReopenPullRequest:
		return "reopen_pull_request"
	case ActionDeleteTag:
		return "delete_tag"
	case ActionDeleteBranch:
		return "delete_branch"
	case ActionMirrorSyncPush:
		return "mirror_sync_push"
	case ActionMirrorSyncCreate:
		return "mirror_sync_create"
	case ActionMirrorSyncDelete:
		return "mirror_sync_delete"
	case ActionApprovePullRequest:
		return "approve_pull_request"
	case ActionRejectPullRequest:
		return "reject_pull_request"
	case ActionCommentPull:
		return "comment_pull"
	case ActionPublishRelease:
		return "publish_release"
	case ActionPullReviewDismissed:
		return "pull_review_dismissed"
	case ActionPullRequestReadyForReview:
		return "pull_request_ready_for_review"
	case ActionAutoMergePullRequest:
		return "auto_merge_pull_request"
	default:
		return "action-" + strconv.Itoa(int(at))
	}
}
