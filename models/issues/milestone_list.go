// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/optional"

	"xorm.io/builder"
)

// MilestoneList is a list of milestones offering additional functionality
type MilestoneList []*Milestone

func (milestones MilestoneList) getMilestoneIDs() []int64 {
	ids := make([]int64, 0, len(milestones))
	for _, ms := range milestones {
		ids = append(ids, ms.ID)
	}
	return ids
}

// FindMilestoneOptions contain options to get milestones
type FindMilestoneOptions struct {
	db.ListOptions
	RepoID   int64
	IsClosed optional.Option[bool]
	Name     string
	SortType string
	RepoCond builder.Cond
	RepoIDs  []int64
}

func (opts FindMilestoneOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.IsClosed.Has() {
		cond = cond.And(builder.Eq{"is_closed": opts.IsClosed.Value()})
	}
	if opts.RepoCond != nil && opts.RepoCond.IsValid() {
		cond = cond.And(builder.In("repo_id", builder.Select("id").From("repository").Where(opts.RepoCond)))
	}
	if len(opts.RepoIDs) > 0 {
		cond = cond.And(builder.In("repo_id", opts.RepoIDs))
	}
	if len(opts.Name) != 0 {
		cond = cond.And(db.BuildCaseInsensitiveLike("name", opts.Name))
	}

	return cond
}

func (opts FindMilestoneOptions) ToOrders() string {
	switch opts.SortType {
	case "furthestduedate":
		return "deadline_unix DESC"
	case "leastcomplete":
		return "completeness ASC"
	case "mostcomplete":
		return "completeness DESC"
	case "leastissues":
		return "num_issues ASC"
	case "mostissues":
		return "num_issues DESC"
	case "id":
		return "id ASC"
	case "name":
		return "name DESC"
	default:
		return "deadline_unix ASC, name ASC"
	}
}

// GetMilestoneIDsByNames returns a list of milestone ids by given names.
// It doesn't filter them by repo, so it could return milestones belonging to different repos.
// It's used for filtering issues via indexer, otherwise it would be useless.
// Since it could return milestones with the same name, so the length of returned ids could be more than the length of names.
func GetMilestoneIDsByNames(ctx context.Context, names []string) ([]int64, error) {
	var ids []int64
	return ids, db.GetEngine(ctx).Table("milestone").
		Where(db.BuildCaseInsensitiveIn("name", names)).
		Cols("id").
		Find(&ids)
}

// LoadTotalTrackedTimes loads for every milestone in the list the TotalTrackedTime by a batch request
func (milestones MilestoneList) LoadTotalTrackedTimes(ctx context.Context) error {
	type totalTimesByMilestone struct {
		MilestoneID int64
		Time        int64
	}
	if len(milestones) == 0 {
		return nil
	}
	trackedTimes := make(map[int64]int64, len(milestones))

	// Get total tracked time by milestone_id
	rows, err := db.GetEngine(ctx).Table("issue").
		Join("INNER", "milestone", "issue.milestone_id = milestone.id").
		Join("LEFT", "tracked_time", "tracked_time.issue_id = issue.id").
		Where("tracked_time.deleted = ?", false).
		Select("milestone_id, sum(time) as time").
		In("milestone_id", milestones.getMilestoneIDs()).
		GroupBy("milestone_id").
		Rows(new(totalTimesByMilestone))
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var totalTime totalTimesByMilestone
		err = rows.Scan(&totalTime)
		if err != nil {
			return err
		}
		trackedTimes[totalTime.MilestoneID] = totalTime.Time
	}

	for _, milestone := range milestones {
		milestone.TotalTrackedTime = trackedTimes[milestone.ID]
	}
	return nil
}

// CountMilestonesByRepoCondAndKw map from repo conditions and the keyword of milestones' name to number of milestones matching the options`
func CountMilestonesMap(ctx context.Context, opts FindMilestoneOptions) (map[int64]int64, error) {
	sess := db.GetEngine(ctx).Where(opts.ToConds())

	countsSlice := make([]*struct {
		RepoID int64
		Count  int64
	}, 0, 10)
	if err := sess.GroupBy("repo_id").
		Select("repo_id AS repo_id, COUNT(*) AS count").
		Table("milestone").
		Find(&countsSlice); err != nil {
		return nil, err
	}

	countMap := make(map[int64]int64, len(countsSlice))
	for _, c := range countsSlice {
		countMap[c.RepoID] = c.Count
	}
	return countMap, nil
}

// MilestonesStats represents milestone statistic information.
type MilestonesStats struct {
	OpenCount, ClosedCount int64
}

// Total returns the total counts of milestones
func (m MilestonesStats) Total() int64 {
	return m.OpenCount + m.ClosedCount
}

// GetMilestonesStatsByRepoCondAndKw returns milestone statistic information for dashboard by given repo conditions and name keyword.
func GetMilestonesStatsByRepoCondAndKw(ctx context.Context, repoCond builder.Cond, keyword string) (*MilestonesStats, error) {
	var err error
	stats := &MilestonesStats{}

	sess := db.GetEngine(ctx).Where("is_closed = ?", false)
	if len(keyword) > 0 {
		sess = sess.And(builder.Like{"UPPER(name)", strings.ToUpper(keyword)})
	}
	if repoCond.IsValid() {
		sess.And(builder.In("repo_id", builder.Select("id").From("repository").Where(repoCond)))
	}
	stats.OpenCount, err = sess.Count(new(Milestone))
	if err != nil {
		return nil, err
	}

	sess = db.GetEngine(ctx).Where("is_closed = ?", true)
	if len(keyword) > 0 {
		sess = sess.And(builder.Like{"UPPER(name)", strings.ToUpper(keyword)})
	}
	if repoCond.IsValid() {
		sess.And(builder.In("repo_id", builder.Select("id").From("repository").Where(repoCond)))
	}
	stats.ClosedCount, err = sess.Count(new(Milestone))
	if err != nil {
		return nil, err
	}

	return stats, nil
}
