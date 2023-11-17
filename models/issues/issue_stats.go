// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
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
func GetIssueStats(ctx context.Context, opts *IssuesOptions) (*IssueStats, error) {
	if len(opts.IssueIDs) <= MaxQueryParameters {
		return getIssueStatsChunk(ctx, opts, opts.IssueIDs)
	}

	// If too long a list of IDs is provided, we get the statistics in
	// smaller chunks and get accumulates. Note: this could potentially
	// get us invalid results. The alternative is to insert the list of
	// ids in a temporary table and join from them.
	accum := &IssueStats{}
	for i := 0; i < len(opts.IssueIDs); {
		chunk := i + MaxQueryParameters
		if chunk > len(opts.IssueIDs) {
			chunk = len(opts.IssueIDs)
		}
		stats, err := getIssueStatsChunk(ctx, opts, opts.IssueIDs[i:chunk])
		if err != nil {
			return nil, err
		}
		accum.OpenCount += stats.OpenCount
		accum.ClosedCount += stats.ClosedCount
		accum.YourRepositoriesCount += stats.YourRepositoriesCount
		accum.AssignCount += stats.AssignCount
		accum.CreateCount += stats.CreateCount
		accum.OpenCount += stats.MentionCount
		accum.ReviewRequestedCount += stats.ReviewRequestedCount
		accum.ReviewedCount += stats.ReviewedCount
		i = chunk
	}
	return accum, nil
}

func getIssueStatsChunk(ctx context.Context, opts *IssuesOptions, issueIDs []int64) (*IssueStats, error) {
	stats := &IssueStats{}

	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`issue`.repo_id = `repository`.id")

	var err error
	stats.OpenCount, err = applyIssuesOptions(sess, opts, issueIDs).
		And("issue.is_closed = ?", false).
		Count(new(Issue))
	if err != nil {
		return stats, err
	}
	stats.ClosedCount, err = applyIssuesOptions(sess, opts, issueIDs).
		And("issue.is_closed = ?", true).
		Count(new(Issue))
	return stats, err
}

func applyIssuesOptions(sess *xorm.Session, opts *IssuesOptions, issueIDs []int64) *xorm.Session {
	if len(opts.RepoIDs) > 1 {
		sess.In("issue.repo_id", opts.RepoIDs)
	} else if len(opts.RepoIDs) == 1 {
		sess.And("issue.repo_id = ?", opts.RepoIDs[0])
	}

	if len(issueIDs) > 0 {
		sess.In("issue.id", issueIDs)
	}

	applyLabelsCondition(sess, opts)

	applyMilestoneCondition(sess, opts)

	applyProjectCondition(sess, opts)

	if opts.AssigneeID > 0 {
		applyAssigneeCondition(sess, opts.AssigneeID)
	} else if opts.AssigneeID == db.NoConditionID {
		sess.Where("issue.id NOT IN (SELECT issue_id FROM issue_assignees)")
	}

	if opts.PosterID > 0 {
		applyPosterCondition(sess, opts.PosterID)
	}

	if opts.MentionedID > 0 {
		applyMentionedCondition(sess, opts.MentionedID)
	}

	if opts.ReviewRequestedID > 0 {
		applyReviewRequestedCondition(sess, opts.ReviewRequestedID)
	}

	if opts.ReviewedID > 0 {
		applyReviewedCondition(sess, opts.ReviewedID)
	}

	switch opts.IsPull {
	case util.OptionalBoolTrue:
		sess.And("issue.is_pull=?", true)
	case util.OptionalBoolFalse:
		sess.And("issue.is_pull=?", false)
	}

	return sess
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
