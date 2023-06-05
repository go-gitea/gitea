// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

// DeleteNotPassedAssignee deletes all assignees who aren't passed via the "assignees" array
func DeleteNotPassedAssignee(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assignees []*user_model.User) (err error) {
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
			if _, _, err := ToggleAssignee(ctx, issue, doer, assignee.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ToggleAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func ToggleAssignee(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assigneeID int64) (removed bool, comment *issues_model.Comment, err error) {
	removed, comment, err = issues_model.ToggleIssueAssignee(ctx, issue, doer, assigneeID)
	if err != nil {
		return
	}

	assignee, err1 := user_model.GetUserByID(ctx, assigneeID)
	if err1 != nil {
		err = err1
		return
	}

	notification.NotifyIssueChangeAssignee(ctx, doer, issue, assignee, removed, comment)

	return removed, comment, err
}

// ReviewRequest add or remove a review request from a user for this PR, and make comment for it.
func ReviewRequest(ctx context.Context, issue *issues_model.Issue, doer, reviewer *user_model.User, isAdd bool) (comment *issues_model.Comment, err error) {
	if isAdd {
		comment, err = issues_model.AddReviewRequest(issue, reviewer, doer)
	} else {
		comment, err = issues_model.RemoveReviewRequest(issue, reviewer, doer)
	}

	if err != nil {
		return nil, err
	}

	if comment != nil {
		notification.NotifyPullRequestReviewRequest(ctx, doer, issue, reviewer, isAdd, comment)
	}

	return comment, err
}

// IsValidReviewRequest Check permission for ReviewRequest
func IsValidReviewRequest(ctx context.Context, reviewer, doer *user_model.User, isAdd bool, issue *issues_model.Issue, permDoer *access_model.Permission) error {
	if reviewer.IsOrganization() {
		return issues_model.ErrNotValidReviewRequest{
			Reason: "Organization can't be added as reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}
	if doer.IsOrganization() {
		return issues_model.ErrNotValidReviewRequest{
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

	lastreview, err := issues_model.GetReviewByIssueIDAndUserID(ctx, issue.ID, reviewer.ID)
	if err != nil && !issues_model.IsErrReviewNotExist(err) {
		return err
	}

	var pemResult bool
	if isAdd {
		pemResult = permReviewer.CanAccessAny(perm.AccessModeRead, unit.TypePullRequests)
		if !pemResult {
			return issues_model.ErrNotValidReviewRequest{
				Reason: "Reviewer can't read",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}

		if doer.ID == issue.PosterID && issue.OriginalAuthorID == 0 && lastreview != nil && lastreview.Type != issues_model.ReviewTypeRequest {
			return nil
		}

		pemResult = doer.ID == issue.PosterID
		if !pemResult {
			pemResult = permDoer.CanAccessAny(perm.AccessModeWrite, unit.TypePullRequests)
		}
		if !pemResult {
			pemResult, err = issues_model.IsOfficialReviewer(ctx, issue, doer)
			if err != nil {
				return err
			}
			if !pemResult {
				return issues_model.ErrNotValidReviewRequest{
					Reason: "Doer can't choose reviewer",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}

		if reviewer.ID == issue.PosterID && issue.OriginalAuthorID == 0 {
			return issues_model.ErrNotValidReviewRequest{
				Reason: "poster of pr can't be reviewer",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}
	} else {
		if lastreview != nil && lastreview.Type == issues_model.ReviewTypeRequest && lastreview.ReviewerID == doer.ID {
			return nil
		}

		pemResult = permDoer.IsAdmin()
		if !pemResult {
			return issues_model.ErrNotValidReviewRequest{
				Reason: "Doer is not admin",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}
	}

	return nil
}

// IsValidTeamReviewRequest Check permission for ReviewRequest Team
func IsValidTeamReviewRequest(ctx context.Context, reviewer *organization.Team, doer *user_model.User, isAdd bool, issue *issues_model.Issue) error {
	if doer.IsOrganization() {
		return issues_model.ErrNotValidReviewRequest{
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
				return issues_model.ErrNotValidReviewRequest{
					Reason: "Reviewing team can't read repo",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}

		doerCanWrite := permission.CanAccessAny(perm.AccessModeWrite, unit.TypePullRequests)
		if !doerCanWrite && doer.ID != issue.PosterID {
			official, err := issues_model.IsOfficialReviewer(ctx, issue, doer)
			if err != nil {
				log.Error("Unable to Check if IsOfficialReviewer for %-v in %-v#%d", doer, issue.Repo, issue.Index)
				return err
			}
			if !official {
				return issues_model.ErrNotValidReviewRequest{
					Reason: "Doer can't choose reviewer",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}
	} else if !permission.IsAdmin() {
		return issues_model.ErrNotValidReviewRequest{
			Reason: "Only admin users can remove team requests. Doer is not admin",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	return nil
}

// TeamReviewRequest add or remove a review request from a team for this PR, and make comment for it.
func TeamReviewRequest(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, reviewer *organization.Team, isAdd bool) (comment *issues_model.Comment, err error) {
	if isAdd {
		comment, err = issues_model.AddTeamReviewRequest(issue, reviewer, doer)
	} else {
		comment, err = issues_model.RemoveTeamReviewRequest(issue, reviewer, doer)
	}

	if err != nil {
		return
	}

	if comment == nil || !isAdd {
		return
	}

	// notify all user in this team
	if err = comment.LoadIssue(ctx); err != nil {
		return
	}

	members, err := organization.GetTeamMembers(ctx, &organization.SearchMembersOptions{
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
		notification.NotifyPullRequestReviewRequest(ctx, doer, issue, member, isAdd, comment)
	}

	return comment, err
}
