// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// SPDX-License-Identifier: MIT

package organization

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

// ResultTimesByRepos is a struct for DB query results
type ResultTimesByRepos struct {
	Name    string
	SumTime int64
}

// ResultTimesByMilestones is a struct for DB query results
type ResultTimesByMilestones struct {
	RepoName     string
	Name         string
	ID           string
	SumTime      int64
	HideRepoName bool
}

// ResultTimesByMembers is a struct for DB query results
type ResultTimesByMembers struct {
	Name    string
	SumTime int64
}

// GetTimesByRepos fetches data from DB to serve TimesByRepos.
func GetTimesByRepos(org *Organization, unixfrom, unixto int64) (results []ResultTimesByRepos, err error) {
	// Get the data from the DB
	err = db.GetEngine(db.DefaultContext).
		Select("repository.name, SUM(tracked_time.time) AS sum_time").
		Table("tracked_time").
		Join("INNER", "issue", "tracked_time.issue_id = issue.id").
		Join("INNER", "repository", "issue.repo_id = repository.id").
		Where(builder.Eq{"repository.owner_id": org.ID}).
		And(builder.Eq{"tracked_time.deleted": false}).
		And(builder.Gte{"tracked_time.created_unix": unixfrom}).
		And(builder.Lte{"tracked_time.created_unix": unixto}).
		GroupBy("repository.id").
		OrderBy("repository.name").
		Find(&results)
	return results, err
}

// GetTimesByMilestones gets the actual data from the DB to serve TimesByMilestones.
func GetTimesByMilestones(org *Organization, unixfrom, unixto int64) (results []ResultTimesByMilestones, err error) {
	err = db.GetEngine(db.DefaultContext).
		Select("repository.name AS repo_name, milestone.name, milestone.id, SUM(tracked_time.time) AS sum_time").
		Table("tracked_time").
		Join("INNER", "issue", "tracked_time.issue_id = issue.id").
		Join("INNER", "repository", "issue.repo_id = repository.id").
		Join("LEFT", "milestone", "issue.milestone_id = milestone.id").
		Where(builder.Eq{"repository.owner_id": org.ID}).
		And(builder.Eq{"tracked_time.deleted": false}).
		And(builder.Gte{"tracked_time.created_unix": unixfrom}).
		And(builder.Lte{"tracked_time.created_unix": unixto}).
		GroupBy("repository.id, milestone.id").
		OrderBy("repository.name, milestone.deadline_unix, milestone.id").
		Find(&results)

	return results, err
}

// getTimesByMembers gets the actual data from the DB to serve TimesByMembers.
func GetTimesByMembers(org *Organization, unixfrom, unixto int64) (results []ResultTimesByMembers, err error) {
	err = db.GetEngine(db.DefaultContext).
		Select("user.name, SUM(tracked_time.time) AS sum_time").
		Table("tracked_time").
		Join("INNER", "issue", "tracked_time.issue_id = issue.id").
		Join("INNER", "repository", "issue.repo_id = repository.id").
		Join("INNER", "user", "tracked_time.user_id = user.id").
		Where(builder.Eq{"repository.owner_id": org.ID}).
		And(builder.Eq{"tracked_time.deleted": false}).
		And(builder.Gte{"tracked_time.created_unix": unixfrom}).
		And(builder.Lte{"tracked_time.created_unix": unixto}).
		GroupBy("user.id").
		OrderBy("sum_time DESC").
		Find(&results)
	return results, err
}
