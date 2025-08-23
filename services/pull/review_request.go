// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

// ReviewRequest add or remove a review request from a user for this PR, and make comment for it.
func ReviewRequest(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, permDoer *access_model.Permission, reviewer *user_model.User, isAdd bool) (comment *issues_model.Comment, err error) {
	err = IsValidReviewRequest(ctx, reviewer, doer, isAdd, issue, permDoer)
	if err != nil {
		return nil, err
	}

	if isAdd {
		comment, err = issues_model.AddReviewRequest(ctx, issue, reviewer, doer)
	} else {
		comment, err = issues_model.RemoveReviewRequest(ctx, issue, reviewer, doer)
	}

	if err != nil {
		return nil, err
	}

	if comment != nil {
		notify_service.PullRequestReviewRequest(ctx, doer, issue, reviewer, isAdd, comment)
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

	lastReview, err := issues_model.GetReviewByIssueIDAndUserID(ctx, issue.ID, reviewer.ID)
	if err != nil && !issues_model.IsErrReviewNotExist(err) {
		return err
	}

	canDoerChangeReviewRequests := CanDoerChangeReviewRequests(ctx, doer, issue.Repo, issue.PosterID)

	if isAdd {
		if !permReviewer.CanAccessAny(perm.AccessModeRead, unit.TypePullRequests) {
			return issues_model.ErrNotValidReviewRequest{
				Reason: "Reviewer can't read",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}

		if reviewer.ID == issue.PosterID && issue.OriginalAuthorID == 0 {
			return issues_model.ErrNotValidReviewRequest{
				Reason: "poster of pr can't be reviewer",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}

		if canDoerChangeReviewRequests {
			return nil
		}

		if doer.ID == issue.PosterID && issue.OriginalAuthorID == 0 && lastReview != nil && lastReview.Type != issues_model.ReviewTypeRequest {
			return nil
		}

		return issues_model.ErrNotValidReviewRequest{
			Reason: "Doer can't choose reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	if canDoerChangeReviewRequests {
		return nil
	}

	if lastReview != nil && lastReview.Type == issues_model.ReviewTypeRequest && lastReview.ReviewerID == doer.ID {
		return nil
	}

	return issues_model.ErrNotValidReviewRequest{
		Reason: "Doer can't remove reviewer",
		UserID: doer.ID,
		RepoID: issue.Repo.ID,
	}
}

// IsValidTeamReviewRequest Check permission for ReviewRequest Team
func IsValidTeamReviewRequest(ctx context.Context, reviewer *org_model.Team, doer *user_model.User, isAdd bool, issue *issues_model.Issue) error {
	if doer.IsOrganization() {
		return issues_model.ErrNotValidReviewRequest{
			Reason: "Organization can't be doer to add reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	canDoerChangeReviewRequests := CanDoerChangeReviewRequests(ctx, doer, issue.Repo, issue.PosterID)

	if isAdd {
		if issue.Repo.IsPrivate {
			hasTeam := org_model.HasTeamRepo(ctx, reviewer.OrgID, reviewer.ID, issue.RepoID)

			if !hasTeam {
				return issues_model.ErrNotValidReviewRequest{
					Reason: "Reviewing team can't read repo",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}

		if canDoerChangeReviewRequests {
			return nil
		}

		return issues_model.ErrNotValidReviewRequest{
			Reason: "Doer can't choose reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	if canDoerChangeReviewRequests {
		return nil
	}

	return issues_model.ErrNotValidReviewRequest{
		Reason: "Doer can't remove reviewer",
		UserID: doer.ID,
		RepoID: issue.Repo.ID,
	}
}

// TeamReviewRequest add or remove a review request from a team for this PR, and make comment for it.
func TeamReviewRequest(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, reviewer *org_model.Team, isAdd bool) (comment *issues_model.Comment, err error) {
	err = IsValidTeamReviewRequest(ctx, reviewer, doer, isAdd, issue)
	if err != nil {
		return nil, err
	}
	if isAdd {
		comment, err = issues_model.AddTeamReviewRequest(ctx, issue, reviewer, doer)
	} else {
		comment, err = issues_model.RemoveTeamReviewRequest(ctx, issue, reviewer, doer)
	}

	if err != nil {
		return nil, err
	}

	if comment == nil || !isAdd {
		return nil, nil
	}

	return comment, teamReviewRequestNotify(ctx, issue, doer, reviewer, isAdd, comment)
}

func ReviewRequestNotify(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, reviewNotifiers []*ReviewRequestNotifier) {
	for _, reviewNotifier := range reviewNotifiers {
		if reviewNotifier.Reviewer != nil {
			notify_service.PullRequestReviewRequest(ctx, issue.Poster, issue, reviewNotifier.Reviewer, reviewNotifier.IsAdd, reviewNotifier.Comment)
		} else if reviewNotifier.ReviewTeam != nil {
			if err := teamReviewRequestNotify(ctx, issue, issue.Poster, reviewNotifier.ReviewTeam, reviewNotifier.IsAdd, reviewNotifier.Comment); err != nil {
				log.Error("teamReviewRequestNotify: %v", err)
			}
		}
	}
}

// teamReviewRequestNotify notify all user in this team
func teamReviewRequestNotify(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, reviewer *org_model.Team, isAdd bool, comment *issues_model.Comment) error {
	// notify all user in this team
	if err := comment.LoadIssue(ctx); err != nil {
		return err
	}

	members, err := org_model.GetTeamMembers(ctx, &org_model.SearchMembersOptions{
		TeamID: reviewer.ID,
	})
	if err != nil {
		return err
	}

	for _, member := range members {
		if member.ID == comment.Issue.PosterID {
			continue
		}
		comment.AssigneeID = member.ID
		notify_service.PullRequestReviewRequest(ctx, doer, issue, member, isAdd, comment)
	}

	return err
}

type ReviewRequestNotifier struct {
	Comment    *issues_model.Comment
	IsAdd      bool
	Reviewer   *user_model.User
	ReviewTeam *org_model.Team
}

// CanDoerChangeReviewRequests returns if the doer can add/remove review requests of a PR
func CanDoerChangeReviewRequests(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, posterID int64) bool {
	if repo.IsArchived {
		return false
	}
	// The poster of the PR can change the reviewers
	if doer.ID == posterID {
		return true
	}

	// The owner of the repo can change the reviewers
	if doer.ID == repo.OwnerID {
		return true
	}

	// Collaborators of the repo can change the reviewers
	isCollaborator, err := repo_model.IsCollaborator(ctx, repo.ID, doer.ID)
	if err != nil {
		log.Error("IsCollaborator: %v", err)
		return false
	}
	if isCollaborator {
		return true
	}

	// If the repo's owner is an organization, members of teams with read permission on pull requests can change reviewers
	if repo.Owner.IsOrganization() {
		teams, err := org_model.GetTeamsWithAccessToAnyRepoUnit(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead, unit.TypePullRequests)
		if err != nil {
			log.Error("GetTeamsWithAccessToRepo: %v", err)
			return false
		}
		for _, team := range teams {
			if !team.UnitEnabled(ctx, unit.TypePullRequests) {
				continue
			}
			isMember, err := org_model.IsTeamMember(ctx, repo.OwnerID, team.ID, doer.ID)
			if err != nil {
				log.Error("IsTeamMember: %v", err)
				continue
			}
			if isMember {
				return true
			}
		}
	}

	return false
}

func init() {
	notify_service.RegisterNotifier(&reviewRequestNotifer{})
}

type reviewRequestNotifer struct {
	notify_service.NullNotifier
}

func (n *reviewRequestNotifer) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	var reviewNotifiers []*ReviewRequestNotifier
	if issue.IsPull && issues_model.HasWorkInProgressPrefix(oldTitle) && !issues_model.HasWorkInProgressPrefix(issue.Title) {
		if err := issue.LoadPullRequest(ctx); err != nil {
			log.Error("IssueChangeTitle: LoadPullRequest: %v", err)
			return
		}

		var err error
		reviewNotifiers, err = RequestCodeOwnersReview(ctx, issue.PullRequest)
		if err != nil {
			log.Error("RequestCodeOwnersReview: %v", err)
		}
	}

	ReviewRequestNotify(ctx, issue, issue.Poster, reviewNotifiers)
}
