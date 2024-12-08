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
	"code.gitea.io/gitea/modules/util"
)

// ToAPIComment converts a issues_model.Comment to the api.Comment format for API usage
func ToAPIComment(ctx context.Context, repo *repo_model.Repository, c *issues_model.Comment) *api.Comment {
	return &api.Comment{
		ID:          c.ID,
		Poster:      ToUser(ctx, c.Poster, nil),
		HTMLURL:     c.HTMLURL(ctx),
		IssueURL:    c.IssueURL(ctx),
		PRURL:       c.PRURL(ctx),
		Body:        c.Content,
		Attachments: ToAPIAttachments(repo, c.Attachments),
		Created:     c.CreatedUnix.AsTime(),
		Updated:     c.UpdatedUnix.AsTime(),
	}
}

// ToTimelineComment converts a issues_model.Comment to the api.TimelineComment format
func ToTimelineComment(ctx context.Context, repo *repo_model.Repository, c *issues_model.Comment, doer *user_model.User) *api.TimelineComment {
	err := c.LoadMilestone(ctx)
	if err != nil {
		log.Error("LoadMilestone: %v", err)
		return nil
	}

	err = c.LoadAssigneeUserAndTeam(ctx)
	if err != nil {
		log.Error("LoadAssigneeUserAndTeam: %v", err)
		return nil
	}

	err = c.LoadResolveDoer(ctx)
	if err != nil {
		log.Error("LoadResolveDoer: %v", err)
		return nil
	}

	err = c.LoadDepIssueDetails(ctx)
	if err != nil {
		log.Error("LoadDepIssueDetails: %v", err)
		return nil
	}

	err = c.LoadTime(ctx)
	if err != nil {
		log.Error("LoadTime: %v", err)
		return nil
	}

	err = c.LoadLabel(ctx)
	if err != nil {
		log.Error("LoadLabel: %v", err)
		return nil
	}

	if c.Content != "" {
		if (c.Type == issues_model.CommentTypeAddTimeManual ||
			c.Type == issues_model.CommentTypeStopTracking ||
			c.Type == issues_model.CommentTypeDeleteTimeManual) &&
			c.Content[0] == '|' {
			// TimeTracking Comments from v1.21 on store the seconds instead of an formatted string
			// so we check for the "|" delimiter and convert new to legacy format on demand
			c.Content = util.SecToTime(c.Content[1:])
		}

		if c.Type == issues_model.CommentTypeChangeTimeEstimate {
			timeSec, _ := util.ToInt64(c.Content)
			c.Content = util.TimeEstimateString(timeSec)
		}
	}

	comment := &api.TimelineComment{
		ID:       c.ID,
		Type:     c.Type.String(),
		Poster:   ToUser(ctx, c.Poster, nil),
		HTMLURL:  c.HTMLURL(ctx),
		IssueURL: c.IssueURL(ctx),
		PRURL:    c.PRURL(ctx),
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
		err = c.Time.LoadAttributes(ctx)
		if err != nil {
			log.Error("Time.LoadAttributes: %v", err)
			return nil
		}

		comment.TrackedTime = ToTrackedTime(ctx, doer, c.Time)
	}

	if c.RefIssueID != 0 {
		issue, err := issues_model.GetIssueByID(ctx, c.RefIssueID)
		if err != nil {
			log.Error("GetIssueByID(%d): %v", c.RefIssueID, err)
			return nil
		}
		comment.RefIssue = ToAPIIssue(ctx, doer, issue)
	}

	if c.RefCommentID != 0 {
		com, err := issues_model.GetCommentByID(ctx, c.RefCommentID)
		if err != nil {
			log.Error("GetCommentByID(%d): %v", c.RefCommentID, err)
			return nil
		}
		err = com.LoadPoster(ctx)
		if err != nil {
			log.Error("LoadPoster: %v", err)
			return nil
		}
		comment.RefComment = ToAPIComment(ctx, repo, com)
	}

	if c.Label != nil {
		var org *user_model.User
		var repo *repo_model.Repository
		if c.Label.BelongsToOrg() {
			var err error
			org, err = user_model.GetUserByID(ctx, c.Label.OrgID)
			if err != nil {
				log.Error("GetUserByID(%d): %v", c.Label.OrgID, err)
				return nil
			}
		}
		if c.Label.BelongsToRepo() {
			var err error
			repo, err = repo_model.GetRepositoryByID(ctx, c.Label.RepoID)
			if err != nil {
				log.Error("GetRepositoryByID(%d): %v", c.Label.RepoID, err)
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
		comment.DependentIssue = ToAPIIssue(ctx, doer, c.DependentIssue)
	}

	return comment
}
