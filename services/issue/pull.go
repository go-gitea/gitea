// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	org_model "gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"

	lru "github.com/hashicorp/golang-lru/v2"
	"xorm.io/builder"
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
func getCodeOwnerRules(ctx context.Context, repo *git.Repository, pr *issues_model.PullRequest) ([]*issues_model.CodeOwnerRule, []string, error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, nil, err
	}

	if err := pr.LoadIssue(ctx); err != nil {
		return nil, nil, err
	}

	pr.Issue.Repo = pr.BaseRepo

	if pr.BaseRepo.IsFork {
		return nil, nil, nil
	}

	commit, err := repo.GetBranchCommit(ctx, pr.BaseBranch)
	if err != nil {
		return nil, nil, err
	}
	return GetCodeOwnerRules(ctx, repo, commit)
}

const maxCodeOwnersFileSize = 3 * 1024 * 1024 // 3M as GitHub

type repoCommitCodeOwnersItem struct {
	content  string
	warnings []string
}

var repoCommitCodeOwnersCache = sync.OnceValue(func() *lru.Cache[string, *repoCommitCodeOwnersItem] {
	cache, _ := lru.New[string, *repoCommitCodeOwnersItem](50)
	return cache
})

func getCodeOwnerRulesContent(ctx context.Context, repo *git.Repository, commit *git.Commit) (string, error) {
	for _, file := range codeOwnerFiles {
		blob, err := commit.GetBlobByPath(ctx, repo, file)
		if errors.Is(err, util.ErrNotExist) {
			continue // no CODEOWNERS at this path, try the next candidate
		} else if err != nil {
			return "", err
		}
		// A truncated CODEOWNERS would silently drop rules, so fail closed rather
		// than evaluate an incomplete file (the gate must not under-enforce).
		if blob.Size(ctx) > maxCodeOwnersFileSize {
			return "", util.ErrorWrap(util.ErrUnprocessableContent, "CODEOWNERS file exceeds the maximum readable size")
		}
		// The file exists but is unreadable: propagate instead of swallowing, so
		// callers can fail closed instead of treating it as "no code owners".
		data, err := blob.GetBlobContent(ctx, maxCodeOwnersFileSize)
		if err != nil {
			log.Error("GetCodeOwnerRules: failed to read CODEOWNERS file %q at commit %s: %v", file, commit.ID, err)
			return "", util.ErrorWrap(util.ErrUnprocessableContent, "failed to read CODEOWNERS file")
		}
		return data, nil
	}
	return "", nil
}

// GetCodeOwnerRules reads CODEOWNERS from the supplied commit.
func GetCodeOwnerRules(ctx context.Context, repo *git.Repository, commit *git.Commit) (rules []*issues_model.CodeOwnerRule, warnings []string, err error) {
	key := fmt.Sprintf("%s:%s", repo.GitRepoManagedID(), commit.ID)
	it, ok := repoCommitCodeOwnersCache().Get(key)
	if !ok {
		content, err := getCodeOwnerRulesContent(ctx, repo, commit)
		if err != nil {
			return nil, nil, err
		}
		it = &repoCommitCodeOwnersItem{content, warnings}
		repoCommitCodeOwnersCache().Add(key, it)
	}
	if len(it.warnings) != 0 {
		return nil, it.warnings, nil
	}
	rules, warnings = issues_model.GetCodeOwnersFromContent(ctx, it.content) // it loads database for users and teams, so we can't cache the rules
	return rules, warnings, nil
}

// Get the matching code owner rules for a given pr + repo combination. The returned
// complete flag is false when rule matching was cut short by the match budget, so
// the returned slice is only a partial set of the matching rules.
func getMatchingCodeOwnerRules(ctx context.Context, pr *issues_model.PullRequest) (matchingRules []*issues_model.CodeOwnerRule, complete bool, err error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, false, err
	}
	repo, closer, err := git.RepositoryFromContextOrOpen(ctx, pr.BaseRepo)
	if err != nil {
		return nil, false, err
	}
	defer closer.Close()

	rules, _, err := getCodeOwnerRules(ctx, repo, pr)
	if err != nil {
		return nil, false, err
	}
	if len(rules) == 0 {
		return nil, true, nil
	}

	// get the mergebase
	mergeBase, err := git.MergeBase(ctx, pr.BaseRepo, git.BranchPrefix+pr.BaseBranch, pr.GetGitHeadRefName())
	if err != nil {
		return nil, false, err
	}

	// https://github.com/go-gitea/gitea/issues/29763, we need to get the files changed
	// between the merge base and the head commit but not the base branch and the head commit
	changedFiles, err := repo.GetFilesChangedBetween(ctx, mergeBase, pr.GetGitHeadRefName())
	if err != nil {
		return nil, false, err
	}

	matchingRules, warnings := MatchCodeOwnerRules(rules, changedFiles)
	complete = len(warnings) == 0
	if !complete {
		log.Warn("CODEOWNERS matching for PR %s#%d exceeded its time budget; some rules were not evaluated", pr.BaseRepo.FullName(), pr.ID)
	}
	return matchingRules, complete, nil
}

// MatchCodeOwnerRules selects matching rules, with the last match winning within each named section.
func MatchCodeOwnerRules(rules []*issues_model.CodeOwnerRule, paths []string) (_ []*issues_model.CodeOwnerRule, warnings []string) {
	selected := make(map[*issues_model.CodeOwnerRule]bool)
	deadline := time.Now().Add(codeOwnerMatchBudget)
	for _, path := range paths {
		sections := make(map[string]*issues_model.CodeOwnerRule)
		for _, rule := range rules {
			if rule.Section == "" && selected[rule] {
				continue
			}
			if time.Now().After(deadline) {
				return nil, []string{"CODEOWNERS matching exceeded its time budget"}
			}
			patternMatched, err := rule.Rule.MatchString(path)
			// err only happens when timeouts, any error can be considered as not matched
			if err != nil {
				return nil, []string{fmt.Sprintf("CODEOWNERS rule %q times out for matching against path %q", rule.Rule.String(), path)}
			}
			ruleMatched := patternMatched != rule.Negative
			if !ruleMatched {
				continue
			}
			if rule.Section == "" {
				selected[rule] = true
			} else {
				sections[rule.Section] = rule
			}
		}
		for _, rule := range sections {
			selected[rule] = true
		}
	}
	matching := make([]*issues_model.CodeOwnerRule, 0, len(selected))
	for _, rule := range rules {
		if selected[rule] {
			matching = append(matching, rule)
		}
	}
	return matching, nil
}

func HasAllRequiredCodeownerReviews(ctx context.Context, pb *git_model.ProtectedBranch, pr *issues_model.PullRequest) bool {
	if !pb.BlockOnCodeownerReviews {
		return true
	}

	matchingRules, complete, err := getMatchingCodeOwnerRules(ctx, pr)
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
		Types:        []issues_model.ReviewType{issues_model.ReviewTypeApprove, issues_model.ReviewTypeReject, issues_model.ReviewTypeRequest},
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
	if pr.IsWorkInProgress(ctx) {
		return nil, nil
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}
	pr.Issue.Repo = pr.BaseRepo

	// The notifier only requests reviews, so a partial (budget-truncated) rule set is
	// acceptable here; the complete flag matters only for the merge gate.
	matchingRules, _, err := getMatchingCodeOwnerRules(ctx, pr)
	if err != nil {
		return nil, err
	}
	if len(matchingRules) == 0 {
		return nil, nil
	}

	return pullRequestCodeOwnersReview(ctx, pr, matchingRules)
}

func pullRequestCodeOwnersReview(ctx context.Context, pr *issues_model.PullRequest, matchingRules []*issues_model.CodeOwnerRule) ([]*ReviewRequestNotifier, error) {
	issue := pr.Issue
	uniqUsers := make(map[int64]*user_model.User)
	uniqTeams := make(map[string]*org_model.Team)
	// TODO: should a user in a team be notified if the user themselves doesn't accept notification? Leave the decisions to the future ....
	watchingPullRequests := make(map[int64]bool)
	for _, rule := range matchingRules {
		if !rule.ReviewRequest.ValueOrDefault(true) {
			continue
		}
		for _, u := range rule.Users {
			if !rule.ReviewRequest.Has() && uniqUsers[u.ID] == nil {
				watching, ok := watchingPullRequests[u.ID]
				if !ok {
					watch, err := repo_model.GetWatch(ctx, u.ID, issue.RepoID)
					if err != nil {
						return nil, err
					}
					watching = watch.IsWatching() && watch.IncludePullRequests
					watchingPullRequests[u.ID] = watching
				}
				if !watching {
					continue
				}
			}
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
		reviewed, err := teamMemberHasReviewed(ctx, pr.IssueID, t)
		if err != nil {
			return nil, err
		}
		if reviewed {
			continue
		}
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

// teamMemberHasReviewed reports whether a team member has a non-dismissed approval or rejection on the issue.
// SubmitReview removes the team's review request in that case, so requesting the team again would mail every member again.
// It doesn't use the latest review of each reviewer: a later single code comment is saved as a comment review but keeps the approval.
func teamMemberHasReviewed(ctx context.Context, issueID int64, team *org_model.Team) (bool, error) {
	return db.GetEngine(ctx).Where(builder.Eq{
		"issue_id":  issueID,
		"dismissed": false,
	}.And(builder.In("type", issues_model.ReviewTypeApprove, issues_model.ReviewTypeReject)).
		And(builder.In("reviewer_id", builder.Select("uid").From("team_user").Where(builder.Eq{"team_id": team.ID})))).
		Exist(new(issues_model.Review))
}
