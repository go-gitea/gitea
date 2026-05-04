// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

func (prInfo *pullRequestViewInfo) prepareMergeBoxIconColor() {
	pull := prInfo.issue.PullRequest
	mergeBoxData := prInfo.MergeBoxData
	statusCheckData := prInfo.StatusCheckData
	switch {
	case pull.HasMerged:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-purple"
	case prInfo.issue.IsClosed, prInfo.workInProgressPrefix != "", pull.IsFilesConflicted():
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-text-light"
	case prInfo.IsPullRequestBroken, mergeBoxData.isBlockedByApprovals, mergeBoxData.isBlockedByRejection,
		mergeBoxData.isBlockedByOfficialReviewRequests, mergeBoxData.isBlockedByOutdatedBranch, mergeBoxData.isBlockedByChangedProtectedFiles:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-red"
	case prInfo.enableStatusCheck && (statusCheckData.RequiredChecksState.IsFailure() || statusCheckData.RequiredChecksState.IsError()):
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-red"
	case prInfo.enableStatusCheck && (statusCheckData.LatestCommitStatus == nil || statusCheckData.RequiredChecksState.IsPending() || statusCheckData.RequiredChecksState.IsWarning()):
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-yellow"
	case mergeBoxData.allowMerge && mergeBoxData.requireSigned && !mergeBoxData.willSign:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-red"
	case pull.IsChecking():
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-yellow"
	case pull.IsEmpty():
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-text-light"
	case pull.IsStatusMergeable():
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-green"
	default:
		prInfo.MergeBoxData.TimelineIconClass = "tw-text-red"
	}
}
