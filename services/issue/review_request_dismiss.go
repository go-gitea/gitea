// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	notify_service "code.gitea.io/gitea/services/notify"
)

const dismissApprovalOnReRequestMessage = "Review request re-submitted, approval review dismissed automatically according to repository settings"

// reviewRequestApprovalDismisser keeps per-request caches for re-request dismissal.
type reviewRequestApprovalDismisser struct {
	issue                 *issues_model.Issue
	isConfigured          bool
	isEnabled             bool
	hasApprovalCache      bool
	reviewsByReviewerID   map[int64][]*issues_model.Review
	teamMemberIDsByTeamID map[int64][]int64
	dismissNotifications  []*dismissReviewNotification
}

type dismissReviewNotification struct {
	doer    *user_model.User
	review  *issues_model.Review
	comment *issues_model.Comment
}

func newReviewRequestApprovalDismisser(issue *issues_model.Issue) *reviewRequestApprovalDismisser {
	return &reviewRequestApprovalDismisser{
		issue:                 issue,
		reviewsByReviewerID:   make(map[int64][]*issues_model.Review),
		teamMemberIDsByTeamID: make(map[int64][]int64),
	}
}

// dismissForUser applies the cached dismissal logic to a single reviewer.
func (d *reviewRequestApprovalDismisser) dismissForUser(ctx context.Context, doer, reviewer *user_model.User) error {
	if reviewer == nil {
		return nil
	}
	return d.dismissForReviewerIDs(ctx, doer, []int64{reviewer.ID})
}

// dismissForTeam applies the cached dismissal logic to all members of a team.
func (d *reviewRequestApprovalDismisser) dismissForTeam(ctx context.Context, doer *user_model.User, reviewerTeam *organization.Team) error {
	if reviewerTeam == nil {
		return nil
	}

	reviewerIDs, ok := d.teamMemberIDsByTeamID[reviewerTeam.ID]
	if !ok {
		members, err := organization.GetTeamMembers(ctx, &organization.SearchMembersOptions{TeamID: reviewerTeam.ID})
		if err != nil {
			return err
		}

		reviewerIDs = make([]int64, 0, len(members))
		for _, member := range members {
			reviewerIDs = append(reviewerIDs, member.ID)
		}
		d.teamMemberIDsByTeamID[reviewerTeam.ID] = reviewerIDs
	}

	return d.dismissForReviewerIDs(ctx, doer, reviewerIDs)
}

// dismissForReviewerIDs dismisses prior approvals for the given reviewers when enabled.
func (d *reviewRequestApprovalDismisser) dismissForReviewerIDs(ctx context.Context, doer *user_model.User, reviewerIDs []int64) error {
	reviewerIDSet := make(map[int64]struct{}, len(reviewerIDs))
	for _, reviewerID := range reviewerIDs {
		if reviewerID > 0 {
			reviewerIDSet[reviewerID] = struct{}{}
		}
	}
	if len(reviewerIDSet) == 0 {
		return nil
	}

	enabled, err := d.ensureConfigured(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	if err := d.ensureApprovalCache(ctx); err != nil {
		return err
	}

	for reviewerID := range reviewerIDSet {
		reviews := d.reviewsByReviewerID[reviewerID]
		for _, review := range reviews {
			if review.Dismissed {
				continue
			}

			if err := issues_model.DismissReview(ctx, review, true); err != nil {
				return err
			}

			comment, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
				Doer:     doer,
				Content:  dismissApprovalOnReRequestMessage,
				Type:     issues_model.CommentTypeDismissReview,
				ReviewID: review.ID,
				Issue:    review.Issue,
				Repo:     review.Issue.Repo,
			})
			if err != nil {
				return err
			}

			review.Dismissed = true
			comment.Review = review
			comment.Poster = doer
			comment.Issue = review.Issue

			d.dismissNotifications = append(d.dismissNotifications, &dismissReviewNotification{
				doer:    doer,
				review:  review,
				comment: comment,
			})
		}
	}

	return nil
}

// notify emits queued dismissal notifications after the write path has completed.
func (d *reviewRequestApprovalDismisser) notify(ctx context.Context) {
	if engine, ok := db.GetEngine(ctx).(interface{ IsInTx() bool }); ok && engine.IsInTx() {
		return
	}

	for _, dismissNotification := range d.dismissNotifications {
		notify_service.PullReviewDismiss(ctx, dismissNotification.doer, dismissNotification.review, dismissNotification.comment)
	}
}

// ensureConfigured loads and caches the branch protection flag for the current issue.
func (d *reviewRequestApprovalDismisser) ensureConfigured(ctx context.Context) (bool, error) {
	if d.isConfigured {
		return d.isEnabled, nil
	}
	d.isConfigured = true

	if d.issue == nil || !d.issue.IsPull {
		d.isEnabled = false
		return false, nil
	}

	if d.issue.PullRequest == nil {
		if err := d.issue.LoadPullRequest(ctx); err != nil {
			return false, err
		}
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, d.issue.PullRequest.BaseRepoID, d.issue.PullRequest.BaseBranch)
	if err != nil {
		return false, err
	}

	d.isEnabled = pb != nil && pb.DismissApprovalsOnReRequest
	return d.isEnabled, nil
}

// ensureApprovalCache loads open approvals once and groups them by reviewer ID.
func (d *reviewRequestApprovalDismisser) ensureApprovalCache(ctx context.Context) error {
	if d.hasApprovalCache {
		return nil
	}
	d.hasApprovalCache = true

	reviews, err := issues_model.FindReviews(ctx, issues_model.FindReviewOptions{
		ListOptions: db.ListOptionsAll,
		IssueID:     d.issue.ID,
		Types:       []issues_model.ReviewType{issues_model.ReviewTypeApprove},
		Dismissed:   optional.Some(false),
	})
	if err != nil {
		return err
	}

	if len(reviews) == 0 {
		return nil
	}

	if err := reviews.LoadIssues(ctx); err != nil {
		return err
	}

	for _, review := range reviews {
		d.reviewsByReviewerID[review.ReviewerID] = append(d.reviewsByReviewerID[review.ReviewerID], review)
	}

	return nil
}
