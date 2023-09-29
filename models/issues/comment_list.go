// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
)

// CommentList defines a list of comments
type CommentList []*Comment

func (comments CommentList) getPosterIDs() []int64 {
	posterIDs := make(container.Set[int64], len(comments))
	for _, comment := range comments {
		posterIDs.Add(comment.PosterID)
	}
	return posterIDs.Values()
}

// LoadPosters loads posters
func (comments CommentList) LoadPosters(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	posterMaps, err := getPosters(ctx, comments.getPosterIDs())
	if err != nil {
		return err
	}

	for _, comment := range comments {
		comment.Poster = getPoster(comment.PosterID, posterMaps)
	}
	return nil
}

func (comments CommentList) getCommentIDs() []int64 {
	ids := make([]int64, 0, len(comments))
	for _, comment := range comments {
		ids = append(ids, comment.ID)
	}
	return ids
}

func (comments CommentList) getLabelIDs() []int64 {
	ids := make(container.Set[int64], len(comments))
	for _, comment := range comments {
		ids.Add(comment.LabelID)
	}
	return ids.Values()
}

func (comments CommentList) loadLabels(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	labelIDs := comments.getLabelIDs()
	commentLabels := make(map[int64]*Label, len(labelIDs))
	left := len(labelIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("id", labelIDs[:limit]).
			Rows(new(Label))
		if err != nil {
			return err
		}

		for rows.Next() {
			var label Label
			err = rows.Scan(&label)
			if err != nil {
				_ = rows.Close()
				return err
			}
			commentLabels[label.ID] = &label
		}
		_ = rows.Close()
		left -= limit
		labelIDs = labelIDs[limit:]
	}

	for _, comment := range comments {
		comment.Label = commentLabels[comment.ID]
	}
	return nil
}

func (comments CommentList) getMilestoneIDs() []int64 {
	ids := make(container.Set[int64], len(comments))
	for _, comment := range comments {
		ids.Add(comment.MilestoneID)
	}
	return ids.Values()
}

func (comments CommentList) loadMilestones(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	milestoneIDs := comments.getMilestoneIDs()
	if len(milestoneIDs) == 0 {
		return nil
	}

	milestoneMaps := make(map[int64]*Milestone, len(milestoneIDs))
	left := len(milestoneIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		err := db.GetEngine(ctx).
			In("id", milestoneIDs[:limit]).
			Find(&milestoneMaps)
		if err != nil {
			return err
		}
		left -= limit
		milestoneIDs = milestoneIDs[limit:]
	}

	for _, issue := range comments {
		issue.Milestone = milestoneMaps[issue.MilestoneID]
	}
	return nil
}

func (comments CommentList) getOldMilestoneIDs() []int64 {
	ids := make(container.Set[int64], len(comments))
	for _, comment := range comments {
		ids.Add(comment.OldMilestoneID)
	}
	return ids.Values()
}

func (comments CommentList) loadOldMilestones(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	milestoneIDs := comments.getOldMilestoneIDs()
	if len(milestoneIDs) == 0 {
		return nil
	}

	milestoneMaps := make(map[int64]*Milestone, len(milestoneIDs))
	left := len(milestoneIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		err := db.GetEngine(ctx).
			In("id", milestoneIDs[:limit]).
			Find(&milestoneMaps)
		if err != nil {
			return err
		}
		left -= limit
		milestoneIDs = milestoneIDs[limit:]
	}

	for _, issue := range comments {
		issue.OldMilestone = milestoneMaps[issue.MilestoneID]
	}
	return nil
}

func (comments CommentList) getAssigneeIDs() []int64 {
	ids := make(container.Set[int64], len(comments))
	for _, comment := range comments {
		ids.Add(comment.AssigneeID)
	}
	return ids.Values()
}

func (comments CommentList) loadAssignees(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	assigneeIDs := comments.getAssigneeIDs()
	assignees := make(map[int64]*user_model.User, len(assigneeIDs))
	left := len(assigneeIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("id", assigneeIDs[:limit]).
			Rows(new(user_model.User))
		if err != nil {
			return err
		}

		for rows.Next() {
			var user user_model.User
			err = rows.Scan(&user)
			if err != nil {
				rows.Close()
				return err
			}

			assignees[user.ID] = &user
		}
		_ = rows.Close()

		left -= limit
		assigneeIDs = assigneeIDs[limit:]
	}

	for _, comment := range comments {
		comment.Assignee = assignees[comment.AssigneeID]
	}
	return nil
}

// getIssueIDs returns all the issue ids on this comment list which issue hasn't been loaded
func (comments CommentList) getIssueIDs() []int64 {
	ids := make(container.Set[int64], len(comments))
	for _, comment := range comments {
		if comment.Issue != nil {
			continue
		}
		ids.Add(comment.IssueID)
	}
	return ids.Values()
}

// Issues returns all the issues of comments
func (comments CommentList) Issues() IssueList {
	issues := make(map[int64]*Issue, len(comments))
	for _, comment := range comments {
		if comment.Issue != nil {
			if _, ok := issues[comment.Issue.ID]; !ok {
				issues[comment.Issue.ID] = comment.Issue
			}
		}
	}

	issueList := make([]*Issue, 0, len(issues))
	for _, issue := range issues {
		issueList = append(issueList, issue)
	}
	return issueList
}

// LoadIssues loads issues of comments
func (comments CommentList) LoadIssues(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	issueIDs := comments.getIssueIDs()
	issues := make(map[int64]*Issue, len(issueIDs))
	left := len(issueIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("id", issueIDs[:limit]).
			Rows(new(Issue))
		if err != nil {
			return err
		}

		for rows.Next() {
			var issue Issue
			err = rows.Scan(&issue)
			if err != nil {
				rows.Close()
				return err
			}

			issues[issue.ID] = &issue
		}
		_ = rows.Close()

		left -= limit
		issueIDs = issueIDs[limit:]
	}

	for _, comment := range comments {
		if comment.Issue == nil {
			comment.Issue = issues[comment.IssueID]
		}
	}
	return nil
}

func (comments CommentList) getDependentIssueIDs() []int64 {
	ids := make(container.Set[int64], len(comments))
	for _, comment := range comments {
		if comment.DependentIssue != nil {
			continue
		}
		ids.Add(comment.DependentIssueID)
	}
	return ids.Values()
}

func (comments CommentList) loadDependentIssues(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	e := db.GetEngine(ctx)
	issueIDs := comments.getDependentIssueIDs()
	issues := make(map[int64]*Issue, len(issueIDs))
	left := len(issueIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := e.
			In("id", issueIDs[:limit]).
			Rows(new(Issue))
		if err != nil {
			return err
		}

		for rows.Next() {
			var issue Issue
			err = rows.Scan(&issue)
			if err != nil {
				_ = rows.Close()
				return err
			}

			issues[issue.ID] = &issue
		}
		_ = rows.Close()

		left -= limit
		issueIDs = issueIDs[limit:]
	}

	for _, comment := range comments {
		if comment.DependentIssue == nil {
			comment.DependentIssue = issues[comment.DependentIssueID]
			if comment.DependentIssue != nil {
				if err := comment.DependentIssue.LoadRepo(ctx); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// LoadAttachments loads attachments
func (comments CommentList) LoadAttachments(ctx context.Context) (err error) {
	if len(comments) == 0 {
		return nil
	}

	attachments := make(map[int64][]*repo_model.Attachment, len(comments))
	commentsIDs := comments.getCommentIDs()
	left := len(commentsIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).Table("attachment").
			Join("INNER", "comment", "comment.id = attachment.comment_id").
			In("comment.id", commentsIDs[:limit]).
			Rows(new(repo_model.Attachment))
		if err != nil {
			return err
		}

		for rows.Next() {
			var attachment repo_model.Attachment
			err = rows.Scan(&attachment)
			if err != nil {
				_ = rows.Close()
				return err
			}
			attachments[attachment.CommentID] = append(attachments[attachment.CommentID], &attachment)
		}

		_ = rows.Close()
		left -= limit
		commentsIDs = commentsIDs[limit:]
	}

	for _, comment := range comments {
		comment.Attachments = attachments[comment.ID]
	}
	return nil
}

func (comments CommentList) getReviewIDs() []int64 {
	ids := make(container.Set[int64], len(comments))
	for _, comment := range comments {
		ids.Add(comment.ReviewID)
	}
	return ids.Values()
}

func (comments CommentList) loadReviews(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	reviewIDs := comments.getReviewIDs()
	reviews := make(map[int64]*Review, len(reviewIDs))
	if err := db.GetEngine(ctx).In("id", reviewIDs).Find(&reviews); err != nil {
		return err
	}

	for _, comment := range comments {
		comment.Review = reviews[comment.ReviewID]
		if comment.Review == nil {
			if comment.ReviewID > 0 {
				log.Error("comment with review id [%d] but has no review record", comment.ReviewID)
			}
			continue
		}

		// If the comment dismisses a review, we need to load the reviewer to show whose review has been dismissed.
		// Otherwise, the reviewer is the poster of the comment, so we don't need to load it.
		if comment.Type == CommentTypeDismissReview {
			if err := comment.Review.LoadReviewer(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadAttributes loads attributes of the comments, except for attachments and
// comments
func (comments CommentList) LoadAttributes(ctx context.Context) (err error) {
	if err = comments.LoadPosters(ctx); err != nil {
		return err
	}

	if err = comments.loadLabels(ctx); err != nil {
		return err
	}

	if err = comments.loadMilestones(ctx); err != nil {
		return err
	}

	if err = comments.loadOldMilestones(ctx); err != nil {
		return err
	}

	if err = comments.loadAssignees(ctx); err != nil {
		return err
	}

	if err = comments.LoadAttachments(ctx); err != nil {
		return err
	}

	if err = comments.loadReviews(ctx); err != nil {
		return err
	}

	if err = comments.LoadIssues(ctx); err != nil {
		return err
	}

	return comments.loadDependentIssues(ctx)
}
