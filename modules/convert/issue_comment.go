// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToComment converts a models.Comment to the api.Comment format
func ToComment(c *models.Comment) *api.Comment {
	return &api.Comment{
		ID:       c.ID,
		Poster:   ToUser(c.Poster, nil),
		HTMLURL:  c.HTMLURL(),
		IssueURL: c.IssueURL(),
		PRURL:    c.PRURL(),
		Body:     c.Content,
		Created:  c.CreatedUnix.AsTime(),
		Updated:  c.UpdatedUnix.AsTime(),
	}
}

// ToTimelineComment converts a models.Comment to the api.TimelineComment format
func ToTimelineComment(c *models.Comment, doer *models.User) *api.TimelineComment {
	err := c.LoadMilestone()
	if err != nil {
		return nil
	}

	err = c.LoadAssigneeUserAndTeam()
	if err != nil {
		return nil
	}

	err = c.LoadResolveDoer()
	if err != nil {
		return nil
	}

	err = c.LoadDepIssueDetails()
	if err != nil {
		return nil
	}

	err = c.LoadTime()
	if err != nil {
		return nil
	}

	err = c.LoadLabel()
	if err != nil {
		return nil
	}

	comment := &api.TimelineComment{
		ID:       c.ID,
		Type:     int64(c.Type),
		Poster:   ToUser(c.Poster, nil),
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

		RefAction:    int64(c.RefAction),
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
		comment.Time = ToTrackedTime(c.Time)
	}

	if c.RefIssueID != 0 {
		issue, err := models.GetIssueByID(c.RefIssueID)
		if err != nil {
			return nil
		}
		comment.RefIssue = ToAPIIssue(issue)
	}

	if c.RefCommentID != 0 {
		com, err := models.GetCommentByID(c.RefCommentID)
		if err != nil {
			return nil
		}
		err = com.LoadPoster()
		if err != nil {
			return nil
		}
		comment.RefComment = ToComment(com)
	}

	if c.Label != nil {
		var org *models.User
		var repo *models.Repository
		if c.Label.BelongsToOrg() {
			var err error
			org, err = models.GetUserByID(c.Label.OrgID)
			if err != nil {
				return nil
			}
		}
		if c.Label.BelongsToRepo() {
			var err error
			repo, err = models.GetRepositoryByID(c.Label.RepoID)
			if err != nil {
				return nil
			}
		}
		comment.Label = ToLabel(c.Label, repo, org)
	}

	if c.Assignee != nil {
		comment.Assignee = ToUser(c.Assignee, nil)
	}
	if c.AssigneeTeam != nil {
		comment.AssigneeTeam = ToTeam(c.AssigneeTeam)
	}

	if c.ResolveDoer != nil {
		comment.ResolveDoer = ToUser(c.ResolveDoer, nil)
	}

	if c.DependentIssue != nil {
		comment.DependentIssue = ToAPIIssue(c.DependentIssue)
	}

	return comment
}
