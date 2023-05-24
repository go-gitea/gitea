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
	if len(opts.IssueIDs) <= MaxQueryParameters {
		return getIssueStatsChunk(opts, opts.IssueIDs)
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
		stats, err := getIssueStatsChunk(opts, opts.IssueIDs[i:chunk])
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

func getIssueStatsChunk(opts *IssuesOptions, issueIDs []int64) (*IssueStats, error) {
	stats := &IssueStats{}

	countSession := func(opts *IssuesOptions, issueIDs []int64) *xorm.Session {
		sess := db.GetEngine(db.DefaultContext).
			Join("INNER", "repository", "`issue`.repo_id = `repository`.id")
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

		if opts.ProjectID > 0 {
			sess.Join("INNER", "project_issue", "issue.id = project_issue.issue_id").
				And("project_issue.project_id=?", opts.ProjectID)
		}

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

	var err error
	stats.OpenCount, err = countSession(opts, issueIDs).
		And("issue.is_closed = ?", false).
		Count(new(Issue))
	if err != nil {
		return stats, err
	}
	stats.ClosedCount, err = countSession(opts, issueIDs).
		And("issue.is_closed = ?", true).
		Count(new(Issue))
	return stats, err
}

// GetUserIssueStats returns issue statistic information for dashboard by given conditions.
func GetUserIssueStats(filterMode int, opts IssuesOptions) (*IssueStats, error) {
	if opts.User == nil {
		return nil, errors.New("issue stats without user")
	}
	if opts.IsPull.IsNone() {
		return nil, errors.New("unaccepted ispull option")
	}

	var err error
	stats := &IssueStats{}

	cond := builder.NewCond()

	cond = cond.And(builder.Eq{"issue.is_pull": opts.IsPull.IsTrue()})

	if len(opts.RepoIDs) > 0 {
		cond = cond.And(builder.In("issue.repo_id", opts.RepoIDs))
	}
	if len(opts.IssueIDs) > 0 {
		cond = cond.And(builder.In("issue.id", opts.IssueIDs))
	}
	if opts.RepoCond != nil {
		cond = cond.And(opts.RepoCond)
	}

	if opts.User != nil {
		cond = cond.And(issuePullAccessibleRepoCond("issue.repo_id", opts.User.ID, opts.Org, opts.Team, opts.IsPull.IsTrue()))
	}

	sess := func(cond builder.Cond) *xorm.Session {
		s := db.GetEngine(db.DefaultContext).
			Join("INNER", "repository", "`issue`.repo_id = `repository`.id").
			Where(cond)
		if len(opts.LabelIDs) > 0 {
			s.Join("INNER", "issue_label", "issue_label.issue_id = issue.id").
				In("issue_label.label_id", opts.LabelIDs)
		}

		if opts.IsArchived != util.OptionalBoolNone {
			s.And(builder.Eq{"repository.is_archived": opts.IsArchived.IsTrue()})
		}
		return s
	}

	switch filterMode {
	case FilterModeAll, FilterModeYourRepositories:
		stats.OpenCount, err = sess(cond).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = sess(cond).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeAssign:
		stats.OpenCount, err = applyAssigneeCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyAssigneeCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeCreate:
		stats.OpenCount, err = applyPosterCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyPosterCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeMention:
		stats.OpenCount, err = applyMentionedCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyMentionedCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeReviewRequested:
		stats.OpenCount, err = applyReviewRequestedCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyReviewRequestedCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeReviewed:
		stats.OpenCount, err = applyReviewedCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyReviewedCondition(sess(cond), opts.User.ID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	}

	cond = cond.And(builder.Eq{"issue.is_closed": opts.IsClosed.IsTrue()})
	stats.AssignCount, err = applyAssigneeCondition(sess(cond), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.CreateCount, err = applyPosterCondition(sess(cond), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.MentionCount, err = applyMentionedCondition(sess(cond), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.YourRepositoriesCount, err = sess(cond).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.ReviewRequestedCount, err = applyReviewRequestedCondition(sess(cond), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.ReviewedCount, err = applyReviewedCondition(sess(cond), opts.User.ID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetRepoIssueStats returns number of open and closed repository issues by given filter mode.
func GetRepoIssueStats(repoID, uid int64, filterMode int, isPull bool) (numOpen, numClosed int64) {
	countSession := func(isClosed, isPull bool, repoID int64) *xorm.Session {
		sess := db.GetEngine(db.DefaultContext).
			Where("is_closed = ?", isClosed).
			And("is_pull = ?", isPull).
			And("repo_id = ?", repoID)

		return sess
	}

	openCountSession := countSession(false, isPull, repoID)
	closedCountSession := countSession(true, isPull, repoID)

	switch filterMode {
	case FilterModeAssign:
		applyAssigneeCondition(openCountSession, uid)
		applyAssigneeCondition(closedCountSession, uid)
	case FilterModeCreate:
		applyPosterCondition(openCountSession, uid)
		applyPosterCondition(closedCountSession, uid)
	}

	openResult, _ := openCountSession.Count(new(Issue))
	closedResult, _ := closedCountSession.Count(new(Issue))

	return openResult, closedResult
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
