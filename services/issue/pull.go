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
		blob, err := commit.GetBlobByPath(file)
		if err != nil {
			continue // no CODEOWNERS at this path, try the next candidate
		}
		// A truncated CODEOWNERS would silently drop rules, so fail closed rather
		// than evaluate an incomplete file (the gate must not under-enforce).
		if blob.Size() > setting.UI.MaxDisplayFileSize {
			return nil, fmt.Errorf("CODEOWNERS file %q exceeds the maximum readable size", file)
		}
		// The file exists but is unreadable: propagate instead of swallowing, so
		// callers can fail closed instead of treating it as "no code owners".
		data, err = blob.GetBlobContent(setting.UI.MaxDisplayFileSize)
		if err != nil {
			return nil, err
		}
		break
	}

	// no code owner file = no one to approve
	if data == "" {
		return nil, nil
	}

	rules, warnings := issues_model.GetCodeOwnersFromContent(ctx, data)
	for _, w := range warnings {
		log.Warn("CODEOWNERS parsing for PR %s#%d: %s", pr.BaseRepo.FullName(), pr.ID, w)
	}
	if len(rules) == 0 {
		return nil, nil
	}

	return rules, nil
}

// Get the matching code owner rules for a given pr + repo combination. The returned
// complete flag is false when rule matching was cut short by the match budget, so
// the returned slice is only a partial set of the matching rules.
func getMatchingCodeOwnerRules(ctx context.Context, repo *git.Repository, pr *issues_model.PullRequest) (matchingRules []*issues_model.CodeOwnerRule, complete bool, err error) {
	rules, err := getCodeOwnerRules(ctx, repo, pr)
	if err != nil {
		return nil, false, err
	}
	if len(rules) == 0 {
		return nil, true, nil
	}

	// get the mergebase
	mergeBase, err := gitrepo.MergeBase(ctx, pr.BaseRepo, git.BranchPrefix+pr.BaseBranch, pr.GetGitHeadRefName())
	if err != nil {
		return nil, false, err
	}

	// https://github.com/go-gitea/gitea/issues/29763, we need to get the files changed
	// between the merge base and the head commit but not the base branch and the head commit
	changedFiles, err := repo.GetFilesChangedBetween(mergeBase, pr.GetGitHeadRefName())
	if err != nil {
		return nil, false, err
	}

	matchingRules = make([]*issues_model.CodeOwnerRule, 0)
	complete = true

	// Bound the total time spent matching rules×files. The per-rule MatchTimeout
	// only caps a single match; without an aggregate budget a crafted CODEOWNERS
	// plus a PR touching many files could still exhaust CPU inside this loop.
	matchDeadline := time.Now().Add(codeOwnerMatchBudget)
ruleLoop:
	for _, rule := range rules {
		for _, f := range changedFiles {
			if time.Now().After(matchDeadline) {
				log.Warn("CODEOWNERS matching for PR %s#%d exceeded its time budget; some rules were not evaluated", pr.BaseRepo.FullName(), pr.ID)
				complete = false
				break ruleLoop
			}
			matched, _ := rule.Rule.MatchString(f) // err only happens when timeouts, any error can be considered as not matched
			if matched != rule.Negative {
				matchingRules = append(matchingRules, rule)
				break
			}
		}
	}

	return matchingRules, complete, nil
}

func HasAllRequiredCodeownerReviews(ctx context.Context, pb *git_model.ProtectedBranch, pr *issues_model.PullRequest) bool {
	if !pb.BlockOnCodeownerReviews {
		return true
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("HasAllRequiredCodeownerReviews: failed to load base repository for PR %d: %v", pr.ID, err)
		return false
	}

	repo, err := gitrepo.OpenRepository(ctx, pr.BaseRepo)
	if err != nil {
		log.Error("HasAllRequiredCodeownerReviews: failed to open base repository for PR %d: %v", pr.ID, err)
		return false
	}

	defer repo.Close()

	matchingRules, complete, err := getMatchingCodeOwnerRules(ctx, repo, pr)
	if err != nil {
		log.Error("HasAllRequiredCodeownerReviews: failed to match code owner rules for PR %d: %v", pr.ID, err)
		return false
	}
	// Rule matching was truncated by the match budget, so matchingRules is only a
	// partial set. Fail closed rather than let an un-evaluated rule pass the gate.
	if !complete {
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
					log.Error("HasAllRequiredCodeownerReviews: failed to load members of team %d for PR %d: %v", t.ID, pr.ID, err)
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

		// A rule whose only code owner is the PR author can never be satisfied (an
		// author cannot approve their own PR), so it is intentionally waived rather
		// than left permanently unmergeable.
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
			return false
		}
	}

	return true
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

	// The notifier only requests reviews, so a partial (budget-truncated) rule set is
	// acceptable here; the complete flag matters only for the merge gate.
	matchingRules, _, err := getMatchingCodeOwnerRules(ctx, repo, pr)
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
