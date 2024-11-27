// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	notify_service "code.gitea.io/gitea/services/notify"
)

func getMergeBase(repo *git.Repository, pr *issues_model.PullRequest, baseBranch, headBranch string) (string, error) {
	// Add a temporary remote
	tmpRemote := fmt.Sprintf("mergebase-%d-%d", pr.ID, time.Now().UnixNano())
	if err := repo.AddRemote(tmpRemote, repo.Path, false); err != nil {
		return "", fmt.Errorf("AddRemote: %w", err)
	}
	defer func() {
		if err := repo.RemoveRemote(tmpRemote); err != nil {
			log.Error("getMergeBase: RemoveRemote: %v", err)
		}
	}()

	mergeBase, _, err := repo.GetMergeBase(tmpRemote, baseBranch, headBranch)
	return mergeBase, err
}

type ReviewRequestNotifier struct {
	Comment    *issues_model.Comment
	IsAdd      bool
	Reviewer   *user_model.User
	ReviewTeam *org_model.Team
}

func RequestCodeOwnersReview(ctx context.Context, issue *issues_model.Issue, pr *issues_model.PullRequest) ([]*ReviewRequestNotifier, error) {
	files := []string{"CODEOWNERS", "docs/CODEOWNERS", ".gitea/CODEOWNERS"}

	if pr.IsWorkInProgress(ctx) {
		return nil, nil
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		return nil, err
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}

	if pr.BaseRepo.IsFork {
		return nil, nil
	}

	repo, err := gitrepo.OpenRepository(ctx, pr.BaseRepo)
	if err != nil {
		return nil, err
	}
	defer repo.Close()

	commit, err := repo.GetBranchCommit(pr.BaseRepo.DefaultBranch)
	if err != nil {
		return nil, err
	}

	var data string
	for _, file := range files {
		if blob, err := commit.GetBlobByPath(file); err == nil {
			data, err = blob.GetBlobContent(setting.UI.MaxDisplayFileSize)
			if err == nil {
				break
			}
		}
	}

	rules, _ := issues_model.GetCodeOwnersFromContent(ctx, data)

	// get the mergebase
	mergeBase, err := getMergeBase(repo, pr, git.BranchPrefix+pr.BaseBranch, pr.GetGitRefName())
	if err != nil {
		return nil, err
	}

	// https://github.com/go-gitea/gitea/issues/29763, we need to get the files changed
	// between the merge base and the head commit but not the base branch and the head commit
	changedFiles, err := repo.GetFilesChangedBetween(mergeBase, pr.GetGitRefName())
	if err != nil {
		return nil, err
	}

	uniqUsers := make(map[int64]*user_model.User)
	uniqTeams := make(map[string]*org_model.Team)
	for _, rule := range rules {
		for _, f := range changedFiles {
			if (rule.Rule.MatchString(f) && !rule.Negative) || (!rule.Rule.MatchString(f) && rule.Negative) {
				for _, u := range rule.Users {
					uniqUsers[u.ID] = u
				}
				for _, t := range rule.Teams {
					uniqTeams[fmt.Sprintf("%d/%d", t.OrgID, t.ID)] = t
				}
			}
		}
	}

	notifiers := make([]*ReviewRequestNotifier, 0, len(uniqUsers)+len(uniqTeams))

	if err := issue.LoadPoster(ctx); err != nil {
		return nil, err
	}

	for _, u := range uniqUsers {
		if u.ID != issue.Poster.ID {
			comment, err := issues_model.AddReviewRequest(ctx, issue, u, issue.Poster)
			if err != nil {
				log.Warn("Failed add assignee user: %s to PR review: %s#%d, error: %s", u.Name, pr.BaseRepo.Name, pr.ID, err)
				return nil, err
			}
			notifiers = append(notifiers, &ReviewRequestNotifier{
				Comment:  comment,
				IsAdd:    true,
				Reviewer: u,
			})
		}
	}
	for _, t := range uniqTeams {
		comment, err := issues_model.AddTeamReviewRequest(ctx, issue, t, issue.Poster)
		if err != nil {
			log.Warn("Failed add assignee team: %s to PR review: %s#%d, error: %s", t.Name, pr.BaseRepo.Name, pr.ID, err)
			return nil, err
		}
		notifiers = append(notifiers, &ReviewRequestNotifier{
			Comment:    comment,
			IsAdd:      true,
			ReviewTeam: t,
		})
	}

	return notifiers, nil
}

// ReviewRequest add or remove a review request from a user for this PR, and make comment for it.
func ReviewRequest(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, permDoer *access_model.Permission, reviewer *user_model.User, isAdd bool) (comment *issues_model.Comment, err error) {
	err = isValidReviewRequest(ctx, reviewer, doer, isAdd, pr.Issue, permDoer)
	if err != nil {
		return nil, err
	}

	if isAdd {
		comment, err = issues_model.AddReviewRequest(ctx, pr.Issue, reviewer, doer)
	} else {
		comment, err = issues_model.RemoveReviewRequest(ctx, pr.Issue, reviewer, doer)
	}

	if err != nil {
		return nil, err
	}

	if comment != nil {
		notify_service.PullRequestReviewRequest(ctx, doer, pr.Issue, reviewer, isAdd, comment)
	}

	return comment, err
}

func ReviewRequests(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, reviewers []*user_model.User, reviewTeams []*org_model.Team) (comments []*issues_model.Comment, err error) {
	for _, reviewer := range reviewers {
		comment, err := ReviewRequest(ctx, pr, doer, nil, reviewer, true)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	for _, reviewTeam := range reviewTeams {
		comment, err := TeamReviewRequest(ctx, pr, doer, reviewTeam, true)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	return comments, nil
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

	if err := issue.LoadRepo(ctx); err != nil {
		return err
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

// isValidTeamReviewRequest Check permission for ReviewRequest Team
func isValidTeamReviewRequest(ctx context.Context, reviewer *org_model.Team, doer *user_model.User, isAdd bool, issue *issues_model.Issue) error {
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
func TeamReviewRequest(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, reviewer *org_model.Team, isAdd bool) (comment *issues_model.Comment, err error) {
	err = isValidTeamReviewRequest(ctx, reviewer, doer, isAdd, pr.Issue)
	if err != nil {
		return nil, err
	}
	if isAdd {
		comment, err = issues_model.AddTeamReviewRequest(ctx, pr.Issue, reviewer, doer)
	} else {
		comment, err = issues_model.RemoveTeamReviewRequest(ctx, pr.Issue, reviewer, doer)
	}

	if err != nil {
		return nil, err
	}

	if comment == nil || !isAdd {
		return nil, nil
	}

	return comment, teamReviewRequestNotify(ctx, pr.Issue, doer, reviewer, isAdd, comment)
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
		teams, err := org_model.GetTeamsWithAccessToRepo(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead)
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
