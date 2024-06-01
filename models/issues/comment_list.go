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

// LoadPosters loads posters
func (comments CommentList) LoadPosters(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	posterIDs := container.FilterSlice(comments, func(c *Comment) (int64, bool) {
		return c.PosterID, c.Poster == nil && c.PosterID > 0
	})

	posterMaps, err := getPostersByIDs(ctx, posterIDs)
	if err != nil {
		return err
	}

	for _, comment := range comments {
		if comment.Poster == nil {
			comment.Poster = getPoster(comment.PosterID, posterMaps)
		}
	}
	return nil
}

func (comments CommentList) getLabelIDs() []int64 {
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.LabelID, comment.LabelID > 0
	})
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
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.MilestoneID, comment.MilestoneID > 0
	})
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
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.OldMilestoneID, comment.OldMilestoneID > 0
	})
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
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.AssigneeID, comment.AssigneeID > 0
	})
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
		if comment.Assignee == nil {
			comment.AssigneeID = user_model.GhostUserID
			comment.Assignee = user_model.NewGhostUser()
		}
	}
	return nil
}

// getIssueIDs returns all the issue ids on this comment list which issue hasn't been loaded
func (comments CommentList) getIssueIDs() []int64 {
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.IssueID, comment.Issue == nil
	})
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
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		if comment.DependentIssue != nil {
			return 0, false
		}
		return comment.DependentIssueID, comment.DependentIssueID > 0
	})
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

// getAttachmentCommentIDs only return the comment ids which possibly has attachments
func (comments CommentList) getAttachmentCommentIDs() []int64 {
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.ID, comment.Type.HasAttachmentSupport()
	})
}

// LoadAttachmentsByIssue loads attachments by issue id
func (comments CommentList) LoadAttachmentsByIssue(ctx context.Context) error {
	if len(comments) == 0 {
		return nil
	}

	attachments := make([]*repo_model.Attachment, 0, len(comments)/2)
	if err := db.GetEngine(ctx).Where("issue_id=? AND comment_id>0", comments[0].IssueID).Find(&attachments); err != nil {
		return err
	}

	commentAttachmentsMap := make(map[int64][]*repo_model.Attachment, len(comments))
	for _, attach := range attachments {
		commentAttachmentsMap[attach.CommentID] = append(commentAttachmentsMap[attach.CommentID], attach)
	}

	for _, comment := range comments {
		comment.Attachments = commentAttachmentsMap[comment.ID]
	}
	return nil
}

// LoadAttachments loads attachments
func (comments CommentList) LoadAttachments(ctx context.Context) (err error) {
	if len(comments) == 0 {
		return nil
	}

	attachments := make(map[int64][]*repo_model.Attachment, len(comments))
	commentsIDs := comments.getAttachmentCommentIDs()
	left := len(commentsIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("comment_id", commentsIDs[:limit]).
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
	return container.FilterSlice(comments, func(comment *Comment) (int64, bool) {
		return comment.ReviewID, comment.ReviewID > 0
	})
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
			// review request which has been replaced by actual reviews doesn't exist in database anymore, so don't log errors for them.
			if comment.ReviewID > 0 && comment.Type != CommentTypeReviewRequest {
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
