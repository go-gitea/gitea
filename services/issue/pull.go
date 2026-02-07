// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"
	"slices"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type ReviewRequestNotifier struct {
	Comment    *issues_model.Comment
	IsAdd      bool
	Reviewer   *user_model.User
	ReviewTeam *org_model.Team
}

var codeOwnerFiles = []string{"CODEOWNERS", "docs/CODEOWNERS", ".gitea/CODEOWNERS"}

func IsCodeOwnerFile(f string) bool {
	return slices.Contains(codeOwnerFiles, f)
}

// Get all code owner rules for a given pr + repo combination.
func getCodeOwnerRules(ctx context.Context, repo *git.Repository, pr *issues_model.PullRequest) ([]*issues_model.CodeOwnerRule, error) {
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return nil, err
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}

	if err := pr.LoadIssue(ctx); err != nil {
		return nil, err
	}

	pr.Issue.Repo = pr.BaseRepo

	if pr.BaseRepo.IsFork {
		return nil, nil
	}

	commit, err := repo.GetBranchCommit(pr.BaseRepo.DefaultBranch)
	if err != nil {
		return nil, err
	}

	var data string

	for _, file := range codeOwnerFiles {
		if blob, err := commit.GetBlobByPath(file); err == nil {
			data, err = blob.GetBlobContent(setting.UI.MaxDisplayFileSize)
			if err == nil {
				break
			}
		}
	}

	// no code owner file = no one to approve
	if data == "" {
		return nil, nil
	}

	rules, _ := issues_model.GetCodeOwnersFromContent(ctx, data)
	if len(rules) == 0 {
		return nil, nil
	}

	return rules, nil
}

// Get the matching code owner rules for a given pr + repo combination.
func getMatchingCodeOwnerRules(ctx context.Context, repo *git.Repository, pr *issues_model.PullRequest) ([]*issues_model.CodeOwnerRule, error) {
	rules, err := getCodeOwnerRules(ctx, repo, pr)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	// get the mergebase
	mergeBase, err := gitrepo.MergeBase(ctx, pr.BaseRepo, git.BranchPrefix+pr.BaseBranch, pr.GetGitHeadRefName())
	if err != nil {
		return nil, err
	}

	// https://github.com/go-gitea/gitea/issues/29763, we need to get the files changed
	// between the merge base and the head commit but not the base branch and the head commit
	changedFiles, err := repo.GetFilesChangedBetween(mergeBase, pr.GetGitHeadRefName())
	if err != nil {
		return nil, err
	}

	matchingRules := make([]*issues_model.CodeOwnerRule, 0)

	for _, rule := range rules {
		for _, f := range changedFiles {
			if (rule.Rule.MatchString(f) && !rule.Negative) || (!rule.Rule.MatchString(f) && rule.Negative) {
				matchingRules = append(matchingRules, rule)
				break
			}
		}
	}

	return matchingRules, nil
}

func HasAllRequiredCodeownerReviews(ctx context.Context, pb *git_model.ProtectedBranch, pr *issues_model.PullRequest) bool {
	if !pb.BlockOnCodeownerReviews {
		return true
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return false
	}

	repo, err := gitrepo.OpenRepository(ctx, pr.BaseRepo)
	if err != nil {
		return true
	}

	defer repo.Close()

	matchingRules, err := getMatchingCodeOwnerRules(ctx, repo, pr)
	if err != nil {
		return false
	}
	if len(matchingRules) == 0 {
		return true
	}

	approvingReviews, err := issues_model.FindLatestReviews(ctx, issues_model.FindReviewOptions{
		Types:   []issues_model.ReviewType{issues_model.ReviewTypeApprove},
		IssueID: pr.IssueID,
	})
	if err != nil {
		return false
	}

	hasApprovals := true

	for _, rule := range matchingRules {
		ruleReviewers := slices.Clone(rule.Users)
		for _, t := range rule.Teams {
			if err := t.LoadMembers(ctx); err != nil {
				return false
			}

			ruleReviewers = slices.AppendSeq(ruleReviewers, slices.Values(t.Members))
		}

		// we need at least 1 code owner that isn't the PR author
		hasPotentialReviewers := slices.ContainsFunc(ruleReviewers, func(elem *user_model.User) bool { return elem.ID != pr.Issue.PosterID })

		if !hasPotentialReviewers {
			continue
		}

		// then we need at least 1 approving review from any valid code owner for this rule
		hasRuleApproval := slices.ContainsFunc(ruleReviewers, func(elem *user_model.User) bool {
			return slices.ContainsFunc(approvingReviews, func(review *issues_model.Review) bool {
				return review.ReviewerID == elem.ID
			})
		})

		if !hasRuleApproval {
			hasApprovals = false
			break
		}
	}

	return hasApprovals
}

func PullRequestCodeOwnersReview(ctx context.Context, pr *issues_model.PullRequest) ([]*ReviewRequestNotifier, error) {
	if err := pr.LoadIssue(ctx); err != nil {
		return nil, err
	}
	issue := pr.Issue
	if pr.IsWorkInProgress(ctx) {
		return nil, nil
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}

	pr.Issue.Repo = pr.BaseRepo

	repo, err := gitrepo.OpenRepository(ctx, pr.BaseRepo)
	if err != nil {
		return nil, err
	}

	defer repo.Close()

	matchingRules, err := getMatchingCodeOwnerRules(ctx, repo, pr)
	if err != nil {
		return nil, err
	}
	if len(matchingRules) == 0 {
		return nil, nil
	}

	uniqUsers := make(map[int64]*user_model.User)
	uniqTeams := make(map[string]*org_model.Team)
	for _, rule := range matchingRules {
		for _, u := range rule.Users {
			uniqUsers[u.ID] = u
		}
		for _, t := range rule.Teams {
			uniqTeams[fmt.Sprintf("%d/%d", t.OrgID, t.ID)] = t
		}
	}

	notifiers := make([]*ReviewRequestNotifier, 0, len(uniqUsers)+len(uniqTeams))

	if err := issue.LoadPoster(ctx); err != nil {
		return nil, err
	}

	// load all reviews from database
	latestReivews, _, err := issues_model.GetReviewsByIssueID(ctx, pr.IssueID)
	if err != nil {
		return nil, err
	}

	contain := func(list issues_model.ReviewList, u *user_model.User) bool {
		for _, review := range list {
			if review.ReviewerTeamID == 0 && review.ReviewerID == u.ID {
				return true
			}
		}
		return false
	}

	for _, u := range uniqUsers {
		if u.ID != issue.Poster.ID && !contain(latestReivews, u) {
			comment, err := issues_model.AddReviewRequest(ctx, issue, u, issue.Poster, true)
			if err != nil {
				log.Warn("Failed add assignee user: %s to PR review: %s#%d, error: %s", u.Name, pr.BaseRepo.Name, pr.ID, err)
				return nil, err
			}
			if comment == nil { // comment maybe nil if review type is ReviewTypeRequest
				continue
			}
			notifiers = append(notifiers, &ReviewRequestNotifier{
				Comment:  comment,
				IsAdd:    true,
				Reviewer: u,
			})
		}
	}

	for _, t := range uniqTeams {
		comment, err := issues_model.AddTeamReviewRequest(ctx, issue, t, issue.Poster, true)
		if err != nil {
			log.Warn("Failed add assignee team: %s to PR review: %s#%d, error: %s", t.Name, pr.BaseRepo.Name, pr.ID, err)
			return nil, err
		}
		if comment == nil { // comment maybe nil if review type is ReviewTypeRequest
			continue
		}
		notifiers = append(notifiers, &ReviewRequestNotifier{
			Comment:    comment,
			IsAdd:      true,
			ReviewTeam: t,
		})
	}

	return notifiers, nil
}
