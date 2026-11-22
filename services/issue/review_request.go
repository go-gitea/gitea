// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"
	"slices"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/services/automergequeue"
	notify_service "gitea.dev/services/notify"
)

// ReviewRequest add or remove a review request from a user for this PR, and make comment for it.
func ReviewRequest(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, permDoer *access_model.Permission, reviewer *user_model.User, isAdd bool) (comment *issues_model.Comment, err error) {
	err = isValidReviewRequest(ctx, reviewer, doer, isAdd, issue, permDoer)
	if err != nil {
		return nil, err
	}

	if isAdd {
		comment, err = issues_model.AddReviewRequest(ctx, issue, reviewer, doer, false)
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

// isValidReviewRequest Check permission for ReviewRequest
func isValidReviewRequest(ctx context.Context, reviewer, doer *user_model.User, isAdd bool, issue *issues_model.Issue, permDoer *access_model.Permission) error {
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

	permReviewer, err := access_model.GetIndividualUserRepoPermission(ctx, issue.Repo, reviewer)
	if err != nil {
		return err
	}

	if permDoer == nil {
		permDoer = new(access_model.Permission)
		*permDoer, err = access_model.GetDoerRepoPermission(ctx, issue.Repo, doer)
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
		if reviewer.ID == issue.PosterID && issue.OriginalAuthorID == 0 {
			return issues_model.ErrNotValidReviewRequest{
				Reason: "poster of pr can't be reviewer",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}

		if canDoerChangeReviewRequests {
			if !isReviewerCandidate(permReviewer, issue.PosterID, reviewer) {
				return issues_model.ErrNotValidReviewRequest{
					Reason: "Reviewer must be a repository member or collaborator with pull request access",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
			return nil
		}

		if !permReviewer.CanAccessAny(perm.AccessModeRead, unit.TypePullRequests) {
			return issues_model.ErrNotValidReviewRequest{
				Reason: "Reviewer can't read",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}

		if doer.ID == issue.PosterID && issue.OriginalAuthorID == 0 && lastReview != nil && lastReview.Type != issues_model.ReviewTypeRequest {
			return nil
		}

		if !issue.Repo.IsArchived && doer.ID == reviewer.ID && lastReview != nil && lastReview.Type == issues_model.ReviewTypeApprove {
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

// isValidTeamReviewRequest Check permission for ReviewRequest Team
func isValidTeamReviewRequest(ctx context.Context, reviewer *organization.Team, doer *user_model.User, isAdd bool, issue *issues_model.Issue) error {
	if doer.IsOrganization() {
		return issues_model.ErrNotValidReviewRequest{
			Reason: "Organization can't be doer to add reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	canDoerChangeReviewRequests := CanDoerChangeReviewRequests(ctx, doer, issue.Repo, issue.PosterID)

	if isAdd {
		if canDoerChangeReviewRequests {
			ok, err := isTeamReviewerCandidate(ctx, issue.Repo, reviewer.ID)
			if err != nil {
				return err
			}
			if !ok {
				return issues_model.ErrNotValidReviewRequest{
					Reason: "Reviewing team must have pull request access to the repository",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
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
func TeamReviewRequest(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, reviewer *organization.Team, isAdd bool) (comment *issues_model.Comment, err error) {
	err = isValidTeamReviewRequest(ctx, reviewer, doer, isAdd, issue)
	if err != nil {
		return nil, err
	}
	if isAdd {
		comment, err = issues_model.AddTeamReviewRequest(ctx, issue, reviewer, doer, false)
	} else {
		comment, err = issues_model.RemoveTeamReviewRequest(ctx, issue, reviewer, doer)
	}

	if err != nil {
		return nil, err
	}

	if comment != nil && !isAdd {
		if err := issue.LoadPullRequest(ctx); err != nil {
			return nil, err
		}
		automergequeue.StartAutoMergeCheckByPullHead(ctx, issue.PullRequest) // team removals don't reach the notifier
	}
	if comment == nil || !isAdd {
		return nil, nil //nolint:nilnil // return nil because no comment was created or it is a removal
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
func teamReviewRequestNotify(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, reviewer *organization.Team, isAdd bool, comment *issues_model.Comment) error {
	// notify all user in this team
	if err := comment.LoadIssue(ctx); err != nil {
		return err
	}

	members, err := organization.GetTeamMembers(ctx, &organization.SearchMembersOptions{
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

// isReviewerCandidate reports whether reviewer may be requested: an active non-poster
// who is a member/collaborator with pull-request read access.
func isReviewerCandidate(permReviewer access_model.Permission, posterID int64, reviewer *user_model.User) bool {
	if reviewer.ID == posterID || !reviewer.IsActive {
		return false
	}
	return permReviewer.IsMemberOrCollaborator() && permReviewer.CanAccess(perm.AccessModeRead, unit.TypePullRequests)
}

// isTeamReviewerCandidate matches services/pull.GetReviewerTeams eligibility.
func isTeamReviewerCandidate(ctx context.Context, repo *repo_model.Repository, teamID int64) (bool, error) {
	if err := repo.LoadOwner(ctx); err != nil {
		return false, fmt.Errorf("isTeamReviewerCandidate: %w", err)
	}
	if !repo.Owner.IsOrganization() {
		return false, nil
	}
	teams, err := organization.GetTeamsWithAccessToAnyRepoUnit(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead, unit.TypePullRequests)
	if err != nil {
		return false, fmt.Errorf("isTeamReviewerCandidate: %w", err)
	}
	return slices.ContainsFunc(teams, func(team *organization.Team) bool {
		return team.ID == teamID
	}), nil
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
		teams, err := organization.GetTeamsWithAccessToAnyRepoUnit(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead, unit.TypePullRequests)
		if err != nil {
			log.Error("GetTeamsWithAccessToRepo: %v", err)
			return false
		}
		for _, team := range teams {
			if !team.UnitEnabled(ctx, unit.TypePullRequests) {
				continue
			}
			isMember, err := organization.IsTeamMember(ctx, repo.OwnerID, team.ID, doer.ID)
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
