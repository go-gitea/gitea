// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"sort"

	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

type WorktimeSumByRepos struct {
	RepoName string
	SumTime  int64
}

func GetWorktimeByRepos(org *Organization, unitFrom, unixTo int64) (results []WorktimeSumByRepos, err error) {
	err = db.GetEngine(db.DefaultContext).
		Select("repository.name AS repo_name, SUM(tracked_time.time) AS sum_time").
		Table("tracked_time").
		Join("INNER", "issue", "tracked_time.issue_id = issue.id").
		Join("INNER", "repository", "issue.repo_id = repository.id").
		Where(builder.Eq{"repository.owner_id": org.ID}).
		And(builder.Eq{"tracked_time.deleted": false}).
		And(builder.Gte{"tracked_time.created_unix": unitFrom}).
		And(builder.Lte{"tracked_time.created_unix": unixTo}).
		GroupBy("repository.name").
		OrderBy("repository.name").
		Find(&results)
	return results, err
}

type WorktimeSumByMilestones struct {
	RepoName          string
	MilestoneName     string
	MilestoneID       int64
	MilestoneDeadline int64
	SumTime           int64
	HideRepoName      bool
}

func GetWorktimeByMilestones(org *Organization, unitFrom, unixTo int64) (results []WorktimeSumByMilestones, err error) {
	err = db.GetEngine(db.DefaultContext).
		Select("repository.name AS repo_name, milestone.name AS milestone_name, milestone.id AS milestone_id, milestone.deadline_unix as milestone_deadline, SUM(tracked_time.time) AS sum_time").
		Table("tracked_time").
		Join("INNER", "issue", "tracked_time.issue_id = issue.id").
		Join("INNER", "repository", "issue.repo_id = repository.id").
		Join("LEFT", "milestone", "issue.milestone_id = milestone.id").
		Where(builder.Eq{"repository.owner_id": org.ID}).
		And(builder.Eq{"tracked_time.deleted": false}).
		And(builder.Gte{"tracked_time.created_unix": unitFrom}).
		And(builder.Lte{"tracked_time.created_unix": unixTo}).
		GroupBy("repository.name, milestone.name, milestone.deadline_unix, milestone.id").
		OrderBy("repository.name, milestone.deadline_unix, milestone.id").
		Find(&results)

	// TODO: pgsql: NULL values are sorted last in default ascending order, so we need to sort them manually again.
	sort.Slice(results, func(i, j int) bool {
		if results[i].RepoName != results[j].RepoName {
			return results[i].RepoName < results[j].RepoName
		}
		if results[i].MilestoneDeadline != results[j].MilestoneDeadline {
			return results[i].MilestoneDeadline < results[j].MilestoneDeadline
		}
		return results[i].MilestoneID < results[j].MilestoneID
	})

	// Show only the first RepoName, for nicer output.
	prevRepoName := ""
	for i := 0; i < len(results); i++ {
		res := &results[i]
		res.MilestoneDeadline = 0 // clear the deadline because we do not really need it
		if prevRepoName == res.RepoName {
			res.HideRepoName = true
		}
		prevRepoName = res.RepoName
	}
	return results, err
}

type WorktimeSumByMembers struct {
	UserName string
	SumTime  int64
}

func GetWorktimeByMembers(org *Organization, unitFrom, unixTo int64) (results []WorktimeSumByMembers, err error) {
	err = db.GetEngine(db.DefaultContext).
		Select("`user`.name AS user_name, SUM(tracked_time.time) AS sum_time").
		Table("tracked_time").
		Join("INNER", "issue", "tracked_time.issue_id = issue.id").
		Join("INNER", "repository", "issue.repo_id = repository.id").
		Join("INNER", "`user`", "tracked_time.user_id = `user`.id").
		Where(builder.Eq{"repository.owner_id": org.ID}).
		And(builder.Eq{"tracked_time.deleted": false}).
		And(builder.Gte{"tracked_time.created_unix": unitFrom}).
		And(builder.Lte{"tracked_time.created_unix": unixTo}).
		GroupBy("`user`.name").
		OrderBy("sum_time DESC").
		Find(&results)
	return results, err
}
