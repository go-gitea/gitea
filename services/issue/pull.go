// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"
	"slices"
	"time"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	org_model "gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
)

type ReviewRequestNotifier struct {
	Comment    *issues_model.Comment
	IsAdd      bool
	Reviewer   *user_model.User
	ReviewTeam *org_model.Team
}

var codeOwnerFiles = []string{"CODEOWNERS", "docs/CODEOWNERS", ".gitea/CODEOWNERS"}

// codeOwnerMatchBudget caps the total wall-clock time spent evaluating all
// CODEOWNERS rules against all changed files for a single PR.
const codeOwnerMatchBudget = 2 * time.Second

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

	commit, err := repo.GetBranchCommit(pr.BaseBranch)
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

	// Bound the total time spent matching rules×files. The per-rule MatchTimeout
	// only caps a single match; without an aggregate budget a crafted CODEOWNERS
	// plus a PR touching many files could still exhaust CPU inside this loop.
	matchDeadline := time.Now().Add(codeOwnerMatchBudget)
ruleLoop:
	for _, rule := range rules {
		for _, f := range changedFiles {
			if time.Now().After(matchDeadline) {
				log.Warn("CODEOWNERS matching for PR %s#%d exceeded its time budget; some rules were not evaluated", pr.BaseRepo.FullName(), pr.ID)
				break ruleLoop
			}
			matched, _ := rule.Rule.MatchString(f) // err only happens when timeouts, any error can be considered as not matched
			if (matched && !rule.Negative) || (!matched && rule.Negative) {
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
		log.Error("HasAllRequiredCodeownerReviews: failed to open base repository: %v", err)
		return false
	}

	defer repo.Close()

	matchingRules, err := getMatchingCodeOwnerRules(ctx, repo, pr)
	if err != nil {
		return false
	}
	if len(matchingRules) == 0 {
		return true
	}

	// OfficialOnly is intentionally false here: a code owner's approval satisfies this gate
	// even if it wouldn't count as an "official" review toward RequiredApprovals (e.g. the
	// owner lacks write access, or isn't on the approvals whitelist). Unlike the other
	// branch-protection checks below, this one only cares whether a listed code owner approved.
	approvingReviews, err := issues_model.FindLatestReviews(ctx, issues_model.FindReviewOptions{
		Types:        []issues_model.ReviewType{issues_model.ReviewTypeApprove, issues_model.ReviewTypeReject},
		IssueID:      pr.IssueID,
		OfficialOnly: false,
		Dismissed:    optional.Some(false),
	})
	if err != nil {
		log.Warn("Failed to get approving reviews for PR review %d, error: %v", pr.ID, err)
		return false
	}

	if pb.IgnoreStaleApprovals {
		validApprovingReviews := make(issues_model.ReviewList, 0, len(approvingReviews))
		for _, review := range approvingReviews {
			if !review.Stale {
				validApprovingReviews = append(validApprovingReviews, review)
			}
		}
		approvingReviews = validApprovingReviews
	}

	hasApprovals := true
	teamMembersByID := make(map[int64][]*user_model.User)

	for _, rule := range matchingRules {
		ruleReviewers := make([]*user_model.User, 0, len(rule.Users))
		for _, u := range rule.Users {
			if u.ID != pr.Issue.PosterID {
				ruleReviewers = append(ruleReviewers, u)
			}
		}
		for _, t := range rule.Teams {
			members, ok := teamMembersByID[t.ID]
			if !ok {
				if err := t.LoadMembers(ctx); err != nil {
					return false
				}
				members = t.Members
				teamMembersByID[t.ID] = members
			}
			for _, m := range members {
				if m.ID != pr.Issue.PosterID {
					ruleReviewers = append(ruleReviewers, m)
				}
			}
		}

		// the rule needs at least 1 code owner that isn't the PR author
		if len(ruleReviewers) == 0 {
			continue
		}

		// and at least 1 approving review from one of them
		hasRuleApproval := slices.ContainsFunc(ruleReviewers, func(elem *user_model.User) bool {
			return slices.ContainsFunc(approvingReviews, func(review *issues_model.Review) bool {
				return review.ReviewerID == elem.ID && review.Type == issues_model.ReviewTypeApprove
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
	latestReviews, _, err := issues_model.GetReviewsByIssueID(ctx, pr.IssueID)
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
		if u.ID != issue.Poster.ID && !contain(latestReviews, u) {
			comment, err := issues_model.AddReviewRequest(ctx, issue, u, issue.Poster, true)
			if err != nil {
				log.Warn("Failed add review user: %s to PR review: %s#%d, error: %s", u.Name, pr.BaseRepo.Name, pr.ID, err)
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
			log.Warn("Failed add reviewer team: %s to PR review: %s#%d, error: %s", t.Name, pr.BaseRepo.Name, pr.ID, err)
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
