// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"
	"slices"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	notify_service "code.gitea.io/gitea/services/notify"
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

func PullRequestCodeOwnersReview(ctx context.Context, pr *issues_model.PullRequest) ([]*ReviewRequestNotifier, error) {
	if err := pr.LoadIssue(ctx); err != nil {
		return nil, err
	}
	issue := pr.Issue
	if pr.IsWorkInProgress(ctx) {
		return nil, nil
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return nil, err
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}
	pr.Issue.Repo = pr.BaseRepo

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
	for _, file := range codeOwnerFiles {
		if blob, err := commit.GetBlobByPath(file); err == nil {
			data, err = blob.GetBlobContent(setting.UI.MaxDisplayFileSize)
			if err == nil {
				break
			}
		}
	}
	if data == "" {
		return nil, nil
	}

	rules, _ := issues_model.GetCodeOwnersFromContent(ctx, data)
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

	uniqUsers := make(map[int64]*user_model.User)
	uniqTeams := make(map[string]*org_model.Team)
	for _, rule := range rules {
		for _, f := range changedFiles {
			shouldMatch := !rule.Negative
			matched, _ := rule.Rule.MatchString(f) // err only happens when timeouts, any error can be considered as not matched
			if matched == shouldMatch {
				for _, u := range rule.Users {
					uniqUsers[u.ID] = u
				}
				for _, t := range rule.Teams {
					uniqTeams[fmt.Sprintf("%d/%d", t.OrgID, t.ID)] = t
				}
			}
		}
	}

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

	dismisser := &reviewRequestDismisser{
		issue:               issue,
		reviewsByReviewerID: make(map[int64][]*issues_model.Review),
	}

	notifiers, err := db.WithTx2(ctx, func(ctx context.Context) ([]*ReviewRequestNotifier, error) {
		notifiers := make([]*ReviewRequestNotifier, 0, len(uniqUsers)+len(uniqTeams))

		for _, u := range uniqUsers {
			if u.ID != issue.Poster.ID && !contain(latestReviews, u) {
				comment, err := issues_model.AddReviewRequest(ctx, issue, u, issue.Poster, true)
				if err != nil {
					log.Warn("Failed add assignee user: %s to PR review: %s#%d, error: %s", u.Name, pr.BaseRepo.Name, pr.ID, err)
					return nil, err
				}
				if comment == nil { // comment maybe nil if review type is ReviewTypeRequest
					continue
				}
				if err := dismisser.dismissReviewsForReviewerIDs(ctx, issue.Poster, []int64{u.ID}); err != nil {
					log.Warn("Failed dismissing prior approvals for user review request: %s to PR review: %s#%d, error: %s", u.Name, pr.BaseRepo.Name, pr.ID, err)
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
			comment, err := issues_model.AddTeamReviewRequest(ctx, issue, t, issue.Poster, true)
			if err != nil {
				log.Warn("Failed add assignee team: %s to PR review: %s#%d, error: %s", t.Name, pr.BaseRepo.Name, pr.ID, err)
				return nil, err
			}
			if comment == nil { // comment maybe nil if review type is ReviewTypeRequest
				continue
			}
			members, err := org_model.GetTeamMembers(ctx, &org_model.SearchMembersOptions{TeamID: t.ID})
			if err != nil {
				log.Warn("Failed dismissing prior approvals for team review request: %s to PR review: %s#%d, error: %s", t.Name, pr.BaseRepo.Name, pr.ID, err)
				return nil, err
			}
			reviewerIDs := make([]int64, 0, len(members))
			for _, m := range members {
				reviewerIDs = append(reviewerIDs, m.ID)
			}
			if err := dismisser.dismissReviewsForReviewerIDs(ctx, issue.Poster, reviewerIDs); err != nil {
				log.Warn("Failed dismissing prior approvals for team review request: %s to PR review: %s#%d, error: %s", t.Name, pr.BaseRepo.Name, pr.ID, err)
				return nil, err
			}
			notifiers = append(notifiers, &ReviewRequestNotifier{
				Comment:    comment,
				IsAdd:      true,
				ReviewTeam: t,
			})
		}

		return notifiers, nil
	})
	if err != nil {
		return nil, err
	}

	if engine, ok := db.GetEngine(ctx).(interface{ IsInTx() bool }); ok && engine.IsInTx() {
		// still in transaction: skip sending notifications
	} else {
		for _, dismissNotification := range dismisser.dismissNotifications {
			notify_service.PullReviewDismiss(ctx, dismissNotification.doer, dismissNotification.review, dismissNotification.comment)
		}
	}

	return notifiers, nil
}
