// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

// DeleteNotPassedAssignee deletes all assignees who aren't passed via the "assignees" array
func DeleteNotPassedAssignee(issue *models.Issue, doer *user_model.User, assignees []*user_model.User) (err error) {
	var found bool
	oriAssignes := make([]*user_model.User, len(issue.Assignees))
	_ = copy(oriAssignes, issue.Assignees)

	for _, assignee := range oriAssignes {
		found = false
		for _, alreadyAssignee := range assignees {
			if assignee.ID == alreadyAssignee.ID {
				found = true
				break
			}
		}

		if !found {
			// This function also does comments and hooks, which is why we call it separately instead of directly removing the assignees here
			if _, _, err := ToggleAssignee(issue, doer, assignee.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ToggleAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func ToggleAssignee(issue *models.Issue, doer *user_model.User, assigneeID int64) (removed bool, comment *models.Comment, err error) {
	removed, comment, err = models.ToggleIssueAssignee(issue, doer, assigneeID)
	if err != nil {
		return
	}

	assignee, err1 := user_model.GetUserByID(assigneeID)
	if err1 != nil {
		err = err1
		return
	}

	notification.NotifyIssueChangeAssignee(doer, issue, assignee, removed, comment)

	return
}

// ReviewRequest add or remove a review request from a user for this PR, and make comment for it.
func ReviewRequest(issue *models.Issue, doer, reviewer *user_model.User, isAdd bool) (comment *models.Comment, err error) {
	if isAdd {
		comment, err = models.AddReviewRequest(issue, reviewer, doer)
	} else {
		comment, err = models.RemoveReviewRequest(issue, reviewer, doer)
	}

	if err != nil {
		return nil, err
	}

	if comment != nil {
		notification.NotifyPullReviewRequest(doer, issue, reviewer, isAdd, comment)
	}

	return
}

// IsValidReviewRequest Check permission for ReviewRequest
func IsValidReviewRequest(ctx context.Context, reviewer, doer *user_model.User, isAdd bool, issue *models.Issue, permDoer *access_model.Permission) error {
	if reviewer.IsOrganization() {
		return models.ErrNotValidReviewRequest{
			Reason: "Organization can't be added as reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}
	if doer.IsOrganization() {
		return models.ErrNotValidReviewRequest{
			Reason: "Organization can't be doer to add reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	permReviewer, err := access_model.GetUserRepoPermission(ctx, issue.Repo, reviewer)
	if err != nil {
		return err
	}

	if permDoer == nil {
		permDoer = new(access_model.Permission)
		*permDoer, err = access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
		if err != nil {
			return err
		}
	}

	lastreview, err := models.GetReviewByIssueIDAndUserID(ctx, issue.ID, reviewer.ID)
	if err != nil && !models.IsErrReviewNotExist(err) {
		return err
	}

	var pemResult bool
	if isAdd {
		pemResult = permReviewer.CanAccessAny(perm.AccessModeRead, unit.TypePullRequests)
		if !pemResult {
			return models.ErrNotValidReviewRequest{
				Reason: "Reviewer can't read",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}

		if doer.ID == issue.PosterID && issue.OriginalAuthorID == 0 && lastreview != nil && lastreview.Type != models.ReviewTypeRequest {
			return nil
		}

		pemResult = permDoer.CanAccessAny(perm.AccessModeWrite, unit.TypePullRequests)
		if !pemResult {
			pemResult, err = models.IsOfficialReviewer(ctx, issue, doer)
			if err != nil {
				return err
			}
			if !pemResult {
				return models.ErrNotValidReviewRequest{
					Reason: "Doer can't choose reviewer",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}

		if reviewer.ID == issue.PosterID && issue.OriginalAuthorID == 0 {
			return models.ErrNotValidReviewRequest{
				Reason: "poster of pr can't be reviewer",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}
	} else {
		if lastreview != nil && lastreview.Type == models.ReviewTypeRequest && lastreview.ReviewerID == doer.ID {
			return nil
		}

		pemResult = permDoer.IsAdmin()
		if !pemResult {
			return models.ErrNotValidReviewRequest{
				Reason: "Doer is not admin",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}
	}

	return nil
}

// IsValidTeamReviewRequest Check permission for ReviewRequest Team
func IsValidTeamReviewRequest(ctx context.Context, reviewer *organization.Team, doer *user_model.User, isAdd bool, issue *models.Issue) error {
	if doer.IsOrganization() {
		return models.ErrNotValidReviewRequest{
			Reason: "Organization can't be doer to add reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	permission, err := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
	if err != nil {
		log.Error("Unable to GetUserRepoPermission for %-v in %-v#%d", doer, issue.Repo, issue.Index)
		return err
	}

	if isAdd {
		if issue.Repo.IsPrivate {
			hasTeam := organization.HasTeamRepo(ctx, reviewer.OrgID, reviewer.ID, issue.RepoID)

			if !hasTeam {
				return models.ErrNotValidReviewRequest{
					Reason: "Reviewing team can't read repo",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}

		doerCanWrite := permission.CanAccessAny(perm.AccessModeWrite, unit.TypePullRequests)
		if !doerCanWrite {
			official, err := models.IsOfficialReviewer(ctx, issue, doer)
			if err != nil {
				log.Error("Unable to Check if IsOfficialReviewer for %-v in %-v#%d", doer, issue.Repo, issue.Index)
				return err
			}
			if !official {
				return models.ErrNotValidReviewRequest{
					Reason: "Doer can't choose reviewer",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}
	} else if !permission.IsAdmin() {
		return models.ErrNotValidReviewRequest{
			Reason: "Only admin users can remove team requests. Doer is not admin",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	return nil
}

// TeamReviewRequest add or remove a review request from a team for this PR, and make comment for it.
func TeamReviewRequest(issue *models.Issue, doer *user_model.User, reviewer *organization.Team, isAdd bool) (comment *models.Comment, err error) {
	if isAdd {
		comment, err = models.AddTeamReviewRequest(issue, reviewer, doer)
	} else {
		comment, err = models.RemoveTeamReviewRequest(issue, reviewer, doer)
	}

	if err != nil {
		return
	}

	if comment == nil || !isAdd {
		return
	}

	// notify all user in this team
	if err = comment.LoadIssue(); err != nil {
		return
	}

	members, err := organization.GetTeamMembers(db.DefaultContext, &organization.SearchMembersOptions{
		TeamID: reviewer.ID,
	})
	if err != nil {
		return
	}

	for _, member := range members {
		if member.ID == comment.Issue.PosterID {
			continue
		}
		comment.AssigneeID = member.ID
		notification.NotifyPullReviewRequest(doer, issue, member, isAdd, comment)
	}

	return
}
