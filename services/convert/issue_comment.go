// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToComment converts a issues_model.Comment to the api.Comment format
func ToComment(ctx context.Context, c *issues_model.Comment) *api.Comment {
	return &api.Comment{
		ID:          c.ID,
		Poster:      ToUser(ctx, c.Poster, nil),
		HTMLURL:     c.HTMLURL(),
		IssueURL:    c.IssueURL(),
		PRURL:       c.PRURL(),
		Body:        c.Content,
		Attachments: ToAttachments(c.Attachments),
		Created:     c.CreatedUnix.AsTime(),
		Updated:     c.UpdatedUnix.AsTime(),
	}
}

// ToTimelineComment converts a issues_model.Comment to the api.TimelineComment format
func ToTimelineComment(ctx context.Context, c *issues_model.Comment, doer *user_model.User) *api.TimelineComment {
	err := c.LoadMilestone(ctx)
	if err != nil {
		log.Error("LoadMilestone: %w", err)
		return nil
	}

	err = c.LoadAssigneeUserAndTeam()
	if err != nil {
		log.Error("LoadAssigneeUserAndTeam: %w", err)
		return nil
	}

	err = c.LoadResolveDoer()
	if err != nil {
		log.Error("LoadResolveDoer: %w", err)
		return nil
	}

	err = c.LoadDepIssueDetails()
	if err != nil {
		log.Error("LoadDepIssueDetails: %w", err)
		return nil
	}

	err = c.LoadTime()
	if err != nil {
		log.Error("LoadTime: %w", err)
		return nil
	}

	err = c.LoadLabel()
	if err != nil {
		log.Error("LoadLabel: %w", err)
		return nil
	}

	comment := &api.TimelineComment{
		ID:       c.ID,
		Type:     c.Type.String(),
		Poster:   ToUser(ctx, c.Poster, nil),
		HTMLURL:  c.HTMLURL(),
		IssueURL: c.IssueURL(),
		PRURL:    c.PRURL(),
		Body:     c.Content,
		Created:  c.CreatedUnix.AsTime(),
		Updated:  c.UpdatedUnix.AsTime(),

		OldProjectID: c.OldProjectID,
		ProjectID:    c.ProjectID,

		OldTitle: c.OldTitle,
		NewTitle: c.NewTitle,

		OldRef: c.OldRef,
		NewRef: c.NewRef,

		RefAction:    c.RefAction.String(),
		RefCommitSHA: c.CommitSHA,

		ReviewID: c.ReviewID,

		RemovedAssignee: c.RemovedAssignee,
	}

	if c.OldMilestone != nil {
		comment.OldMilestone = ToAPIMilestone(c.OldMilestone)
	}
	if c.Milestone != nil {
		comment.Milestone = ToAPIMilestone(c.Milestone)
	}

	if c.Time != nil {
		err = c.Time.LoadAttributes()
		if err != nil {
			log.Error("Time.LoadAttributes: %w", err)
			return nil
		}

		comment.TrackedTime = ToTrackedTime(ctx, c.Time)
	}

	if c.RefIssueID != 0 {
		issue, err := issues_model.GetIssueByID(ctx, c.RefIssueID)
		if err != nil {
			log.Error("GetIssueByID(%d): %w", c.RefIssueID, err)
			return nil
		}
		comment.RefIssue = ToAPIIssue(ctx, issue)
	}

	if c.RefCommentID != 0 {
		com, err := issues_model.GetCommentByID(ctx, c.RefCommentID)
		if err != nil {
			log.Error("GetCommentByID(%d): %w", c.RefCommentID, err)
			return nil
		}
		err = com.LoadPoster(ctx)
		if err != nil {
			log.Error("LoadPoster: %w", err)
			return nil
		}
		comment.RefComment = ToComment(ctx, com)
	}

	if c.Label != nil {
		var org *user_model.User
		var repo *repo_model.Repository
		if c.Label.BelongsToOrg() {
			var err error
			org, err = user_model.GetUserByID(ctx, c.Label.OrgID)
			if err != nil {
				log.Error("GetUserByID(%d): %w", c.Label.OrgID, err)
				return nil
			}
		}
		if c.Label.BelongsToRepo() {
			var err error
			repo, err = repo_model.GetRepositoryByID(ctx, c.Label.RepoID)
			if err != nil {
				log.Error("GetRepositoryByID(%d): %w", c.Label.RepoID, err)
				return nil
			}
		}
		comment.Label = ToLabel(c.Label, repo, org)
	}

	if c.Assignee != nil {
		comment.Assignee = ToUser(ctx, c.Assignee, nil)
	}
	if c.AssigneeTeam != nil {
		comment.AssigneeTeam, _ = ToTeam(ctx, c.AssigneeTeam)
	}

	if c.ResolveDoer != nil {
		comment.ResolveDoer = ToUser(ctx, c.ResolveDoer, nil)
	}

	if c.DependentIssue != nil {
		comment.DependentIssue = ToAPIIssue(ctx, c.DependentIssue)
	}

	return comment
}
