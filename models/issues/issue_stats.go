// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// IssueStats represents issue statistic information.
type IssueStats struct {
	OpenCount, ClosedCount int64
	YourRepositoriesCount  int64
	AssignCount            int64
	CreateCount            int64
	MentionCount           int64
	ReviewRequestedCount   int64
	ReviewedCount          int64
}

// Filter modes.
const (
	FilterModeAll = iota
	FilterModeAssign
	FilterModeCreate
	FilterModeMention
	FilterModeReviewRequested
	FilterModeReviewed
	FilterModeYourRepositories
)

const (
	// MaxQueryParameters represents the max query parameters
	// When queries are broken down in parts because of the number
	// of parameters, attempt to break by this amount
	MaxQueryParameters = 300
)

// CountIssuesByRepo map from repoID to number of issues matching the options
func CountIssuesByRepo(ctx context.Context, opts *IssuesOptions) (map[int64]int64, error) {
	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`issue`.repo_id = `repository`.id")

	applyConditions(sess, opts)

	countsSlice := make([]*struct {
		RepoID int64
		Count  int64
	}, 0, 10)
	if err := sess.GroupBy("issue.repo_id").
		Select("issue.repo_id AS repo_id, COUNT(*) AS count").
		Table("issue").
		Find(&countsSlice); err != nil {
		return nil, fmt.Errorf("unable to CountIssuesByRepo: %w", err)
	}

	countMap := make(map[int64]int64, len(countsSlice))
	for _, c := range countsSlice {
		countMap[c.RepoID] = c.Count
	}
	return countMap, nil
}

// CountIssues number return of issues by given conditions.
func CountIssues(ctx context.Context, opts *IssuesOptions) (int64, error) {
	sess := db.GetEngine(ctx).
		Select("COUNT(issue.id) AS count").
		Table("issue").
		Join("INNER", "repository", "`issue`.repo_id = `repository`.id")
	applyConditions(sess, opts)

	return sess.Count()
}

// GetIssueStats returns issue statistic information by given conditions.
func GetIssueStats(opts *IssuesOptions) (*IssueStats, error) {
	stats := &IssueStats{}

	countSession := func(opts *IssuesOptions) *xorm.Session {
		sess := db.GetEngine(db.DefaultContext).
			Join("INNER", "repository", "`issue`.repo_id = `repository`.id")

		applyRepoConditions(sess, opts)

		applyIsPullCondition(sess, opts)

		applyMilestoneCondition(sess, opts)

		applyPosterCondition(sess, opts.PosterID)

		applyKeywordCondition(sess, opts)

		applyLabelsCondition(sess, opts)

		applyProjectConditions(sess, opts)

		applyAssigneeCondition(sess, opts.AssigneeID)

		applyMentionedCondition(sess, opts.MentionedID)

		applyReviewRequestedCondition(sess, opts.ReviewRequestedID)

		applyReviewedCondition(sess, opts.ReviewedID)

		return sess
	}

	var err error
	stats.OpenCount, err = countSession(opts).
		And("issue.is_closed = ?", false).
		Count(new(Issue))
	if err != nil {
		return stats, err
	}
	stats.ClosedCount, err = countSession(opts).
		And("issue.is_closed = ?", true).
		Count(new(Issue))
	return stats, err
}

// GetUserIssueStats returns issue statistic information for dashboard by given conditions.
func GetUserIssueStats(filterMode int, opts *IssuesOptions) (*IssueStats, error) {
	if opts.User == nil {
		return nil, errors.New("issue stats without user")
	}
	if opts.IsPull.IsNone() {
		return nil, errors.New("unaccepted ispull option")
	}

	stats := &IssueStats{}

	sess := func(isClosed bool) *xorm.Session {
		s := db.GetEngine(db.DefaultContext).
			Join("INNER", "repository", "`issue`.repo_id = `repository`.id").
			And("issue.is_closed = ?", isClosed)

		applyIsPullCondition(s, opts)
		applyRepoConditions(s, opts)
		applyUserCondition(s, opts)
		if len(opts.LabelIDs) > 0 {
			s.Join("INNER", "issue_label", "issue_label.issue_id = issue.id").
				In("issue_label.label_id", opts.LabelIDs)
		}

		applyKeywordCondition(s, opts)

		if opts.IsArchived != util.OptionalBoolNone {
			s.And(builder.Eq{"repository.is_archived": opts.IsArchived.IsTrue()})
		}
		return s
	}

	var err error
	switch filterMode {
	case FilterModeAll, FilterModeYourRepositories:
		stats.OpenCount, err = sess(false).Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = sess(true).Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeAssign:
		stats.OpenCount, err = applyAssigneeCondition(sess(false), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyAssigneeCondition(sess(true), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeCreate:
		stats.OpenCount, err = applyPosterCondition(sess(false), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyPosterCondition(sess(true), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeMention:
		stats.OpenCount, err = applyMentionedCondition(sess(false), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyMentionedCondition(sess(true), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeReviewRequested:
		stats.OpenCount, err = applyReviewRequestedCondition(sess(false), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyReviewRequestedCondition(sess(true), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeReviewed:
		stats.OpenCount, err = applyReviewedCondition(sess(false), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyReviewedCondition(sess(true), opts.User.ID).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	}

	stats.AssignCount, err = applyAssigneeCondition(sess(opts.IsClosed.IsTrue()), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.CreateCount, err = applyPosterCondition(sess(opts.IsClosed.IsTrue()), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.MentionCount, err = applyMentionedCondition(sess(opts.IsClosed.IsTrue()), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.YourRepositoriesCount, err = sess(opts.IsClosed.IsTrue()).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.ReviewRequestedCount, err = applyReviewRequestedCondition(sess(opts.IsClosed.IsTrue()), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.ReviewedCount, err = applyReviewedCondition(sess(opts.IsClosed.IsTrue()), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// CountOrphanedIssues count issues without a repo
func CountOrphanedIssues(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).
		Table("issue").
		Join("LEFT", "repository", "issue.repo_id=repository.id").
		Where(builder.IsNull{"repository.id"}).
		Select("COUNT(`issue`.`id`)").
		Count()
}
