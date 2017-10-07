// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	"github.com/go-xorm/builder"
	"github.com/go-xorm/xorm"
)

// PulseStats represets issue and pull request information.
type PulseStats struct {
	OpenedPRs              PullRequestList
	OpenedPRAuthorCount    int64
	MergedPRs              PullRequestList
	MergedPRAuthorCount    int64
	OpenedIssues           IssueList
	OpenedIssueAuthorCount int64
	ClosedIssues           IssueList
	ClosedIssueAuthorCount int64
	UnresolvedIssues       IssueList
}

// ActivePRCount returns total active pull request count
func (stats *PulseStats) ActivePRCount() int {
	return stats.OpenedPRCount() + stats.MergedPRCount()
}

// OpenedPRCount returns opened pull request count
func (stats *PulseStats) OpenedPRCount() int {
	return len(stats.OpenedPRs)
}

// OpenedPRPerc returns opened pull request percents from total active
func (stats *PulseStats) OpenedPRPerc() int {
	return int(float32(stats.OpenedPRCount()) / float32(stats.ActivePRCount()) * 100.0)
}

// MergedPRCount returns merged pull request count
func (stats *PulseStats) MergedPRCount() int {
	return len(stats.MergedPRs)
}

// MergedPRPerc returns merged pull request percent from total active
func (stats *PulseStats) MergedPRPerc() int {
	return int(float32(stats.MergedPRCount()) / float32(stats.ActivePRCount()) * 100.0)
}

// ActiveIssueCount returns total active issue count
func (stats *PulseStats) ActiveIssueCount() int {
	return stats.OpenedIssueCount() + stats.ClosedIssueCount()
}

// OpenedIssueCount returns open issue count
func (stats *PulseStats) OpenedIssueCount() int {
	return len(stats.OpenedIssues)
}

// OpenedIssuePerc returns open issue count percent from total active
func (stats *PulseStats) OpenedIssuePerc() int {
	return int(float32(stats.OpenedIssueCount()) / float32(stats.ActiveIssueCount()) * 100.0)
}

// ClosedIssueCount returns closed issue count
func (stats *PulseStats) ClosedIssueCount() int {
	return len(stats.ClosedIssues)
}

// ClosedIssuePerc returns closed issue count percent from total active
func (stats *PulseStats) ClosedIssuePerc() int {
	return int(float32(stats.ClosedIssueCount()) / float32(stats.ActiveIssueCount()) * 100.0)
}

// UnresolvedIssueCount returns unresolved issue and pull request count
func (stats *PulseStats) UnresolvedIssueCount() int {
	return len(stats.UnresolvedIssues)
}

// FillPullRequestsForPulse returns pull request information for pulse page
func FillPullRequestsForPulse(stats *PulseStats, baseRepoID int64, fromTime time.Time) error {
	var err error
	row := &struct {
		Count int64
	}{}

	// Merged pull requests
	sess := pullRequestsForPulseStatement(baseRepoID, fromTime, true, false)
	sess.OrderBy("pull_request.merged_unix DESC")
	stats.MergedPRs = make(PullRequestList, 0)
	if err = sess.Find(&stats.MergedPRs); err != nil {
		return err
	}
	if err = stats.MergedPRs.LoadAttributes(); err != nil {
		return err
	}

	// Merged pull request authors
	sess = pullRequestsForPulseStatement(baseRepoID, fromTime, true, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("pull_request").Get(row); err != nil {
		return err
	}
	stats.MergedPRAuthorCount = row.Count

	// Opened pull requests
	sess = pullRequestsForPulseStatement(baseRepoID, fromTime, false, true)
	sess.OrderBy("issue.created_unix ASC")
	stats.OpenedPRs = make(PullRequestList, 0)
	if err := sess.Find(&stats.OpenedPRs); err != nil {
		return err
	}
	if err = stats.OpenedPRs.LoadAttributes(); err != nil {
		return err
	}

	// Opened pull request authors
	row.Count = 0
	sess = pullRequestsForPulseStatement(baseRepoID, fromTime, false, true)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("pull_request").Get(row); err != nil {
		return err
	}
	stats.OpenedPRAuthorCount = row.Count

	return nil
}

func pullRequestsForPulseStatement(baseRepoID int64, fromTime time.Time, merged, created bool) *xorm.Session {
	sess := x.Where("pull_request.base_repo_id=?", baseRepoID).
		Join("INNER", "issue", "pull_request.issue_id = issue.id")

	var mergedCond, createdCond builder.Cond

	if merged {
		mergedCond = builder.Eq{"pull_request.has_merged": true}.
			And(builder.Gte{"pull_request.merged_unix": fromTime.Unix()})
	}
	if created {
		createdCond = builder.Eq{"issue.is_closed": false}.
			And(builder.Gte{"issue.created_unix": fromTime.Unix()})
	}
	if merged && created {
		sess.And(builder.Or(mergedCond, createdCond))
	} else if merged {
		sess.And(mergedCond)
	} else if created {
		sess.And(createdCond)
	}

	return sess
}

// FillIssuesForPulse returns issue information for pulse page
func FillIssuesForPulse(stats *PulseStats, baseRepoID int64, fromTime time.Time) error {
	var err error
	row := &struct {
		Count int64
	}{}

	// Closed issues
	sess := issuesForPulseStatement(baseRepoID, fromTime, true, false)
	sess.OrderBy("issue.updated_unix DESC")
	stats.ClosedIssues = make(IssueList, 0)
	if err = sess.Find(&stats.ClosedIssues); err != nil {
		return err
	}

	// Closed issue authors
	sess = issuesForPulseStatement(baseRepoID, fromTime, true, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("issue").Get(row); err != nil {
		return err
	}
	stats.ClosedIssueAuthorCount = row.Count

	// New issues
	sess = issuesForPulseStatement(baseRepoID, fromTime, false, false)
	sess.OrderBy("issue.created_unix ASC")
	stats.OpenedIssues = make(IssueList, 0)
	if err := sess.Find(&stats.OpenedIssues); err != nil {
		return err
	}

	// Opened issue authors
	row.Count = 0
	sess = issuesForPulseStatement(baseRepoID, fromTime, false, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("issue").Get(row); err != nil {
		return err
	}
	stats.OpenedIssueAuthorCount = row.Count

	return nil
}

// FillUnresolvedIssuesForPulse returns unresolved issue and pull request information for pulse page
func FillUnresolvedIssuesForPulse(stats *PulseStats, baseRepoID int64, fromTime time.Time, issues, prs bool) error {
	// Check if we need to select anything
	if !issues && !prs {
		return nil
	}
	sess := issuesForPulseStatement(baseRepoID, fromTime, false, true)
	if !issues || !prs {
		sess.And("issue.is_pull=?", prs)
	}
	sess.OrderBy("issue.updated_unix DESC")
	stats.UnresolvedIssues = make(IssueList, 0)
	return sess.Find(&stats.UnresolvedIssues)
}

func issuesForPulseStatement(baseRepoID int64, fromTime time.Time, closed, unresolved bool) *xorm.Session {
	sess := x.Where("issue.repo_id=?", baseRepoID).
		And("issue.is_closed=?", closed)

	if !unresolved {
		sess.And("issue.is_pull=?", false)
		sess.And("issue.created_unix >= ?", fromTime.Unix())
	} else {
		sess.And("issue.created_unix < ?", fromTime.Unix())
		sess.And("issue.updated_unix >= ?", fromTime.Unix())
	}

	return sess
}
