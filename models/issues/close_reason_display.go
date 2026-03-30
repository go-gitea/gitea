// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import "encoding/json"

type closeReasonParam struct {
	IssueIndex int64  `json:"issue_index"`
	CommentID  int64  `json:"comment_id"`
	CommitHash string `json:"commit_hash"`
	PullIndex  int64  `json:"pull_index"`
}

func parseCloseReasonParam(param string) closeReasonParam {
	if param == "" {
		return closeReasonParam{}
	}
	var p closeReasonParam
	_ = json.Unmarshal([]byte(param), &p)
	return p
}

func normalizeCloseReason(isClosed bool, reason IssueCloseReason) string {
	if isClosed && reason == IssueCloseReasonNone {
		return IssueCloseReasonCompleted.String()
	}
	return reason.String()
}

func (issue *Issue) CloseReasonForDisplay() string {
	return normalizeCloseReason(issue.IsClosed, issue.CloseReason)
}

func (issue *Issue) CloseReasonDuplicateIssueIndex() int64 {
	return parseCloseReasonParam(issue.CloseReasonParam).IssueIndex
}

func (issue *Issue) CloseReasonAnsweredCommentID() int64 {
	return parseCloseReasonParam(issue.CloseReasonParam).CommentID
}

func (issue *Issue) CloseReasonCommitHash() string {
	return parseCloseReasonParam(issue.CloseReasonParam).CommitHash
}

func (issue *Issue) CloseReasonPullIndex() int64 {
	return parseCloseReasonParam(issue.CloseReasonParam).PullIndex
}

func (comment *Comment) CloseReasonForDisplay() string {
	if comment.CommentMetaData == nil {
		return ""
	}
	return normalizeCloseReason(true, comment.CommentMetaData.CloseReason)
}

func (comment *Comment) CloseReasonDuplicateIssueIndex() int64 {
	if comment.CommentMetaData == nil {
		return 0
	}
	return parseCloseReasonParam(comment.CommentMetaData.CloseReasonParam).IssueIndex
}

func (comment *Comment) CloseReasonAnsweredCommentID() int64 {
	if comment.CommentMetaData == nil {
		return 0
	}
	return parseCloseReasonParam(comment.CommentMetaData.CloseReasonParam).CommentID
}

func (comment *Comment) CloseReasonCommitHash() string {
	if comment.CommentMetaData == nil {
		return ""
	}
	return parseCloseReasonParam(comment.CommentMetaData.CloseReasonParam).CommitHash
}

func (comment *Comment) CloseReasonPullIndex() int64 {
	if comment.CommentMetaData == nil {
		return 0
	}
	return parseCloseReasonParam(comment.CommentMetaData.CloseReasonParam).PullIndex
}
