// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

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

// GetMilestonesOption contain options to get milestones
type GetMilestonesOption struct {
	db.ListOptions
	RepoID   int64
	State    api.StateType
	Name     string
	SortType string
}

func (opts GetMilestonesOption) toCond() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	switch opts.State {
	case api.StateClosed:
		cond = cond.And(builder.Eq{"is_closed": true})
	case api.StateAll:
		break
	// api.StateOpen:
	default:
		cond = cond.And(builder.Eq{"is_closed": false})
	}

	if len(opts.Name) != 0 {
		cond = cond.And(db.BuildCaseInsensitiveLike("name", opts.Name))
	}

	return cond
}

// GetMilestones returns milestones filtered by GetMilestonesOption's
func GetMilestones(ctx context.Context, opts GetMilestonesOption) (MilestoneList, int64, error) {
	sess := db.GetEngine(ctx).Where(opts.toCond())

	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, &opts)
	}

	switch opts.SortType {
	case "furthestduedate":
		sess.Desc("deadline_unix")
	case "leastcomplete":
		sess.Asc("completeness")
	case "mostcomplete":
		sess.Desc("completeness")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	case "id":
		sess.Asc("id")
	default:
		sess.Asc("deadline_unix").Asc("id")
	}

	miles := make([]*Milestone, 0, opts.PageSize)
	total, err := sess.FindAndCount(&miles)
	return miles, total, err
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

// SearchMilestones search milestones
func SearchMilestones(ctx context.Context, repoCond builder.Cond, page int, isClosed bool, sortType, keyword string) (MilestoneList, error) {
	miles := make([]*Milestone, 0, setting.UI.IssuePagingNum)
	sess := db.GetEngine(ctx).Where("is_closed = ?", isClosed)
	if len(keyword) > 0 {
		sess = sess.And(builder.Like{"UPPER(name)", strings.ToUpper(keyword)})
	}
	if repoCond.IsValid() {
		sess.In("repo_id", builder.Select("id").From("repository").Where(repoCond))
	}
	if page > 0 {
		sess = sess.Limit(setting.UI.IssuePagingNum, (page-1)*setting.UI.IssuePagingNum)
	}

	switch sortType {
	case "furthestduedate":
		sess.Desc("deadline_unix")
	case "leastcomplete":
		sess.Asc("completeness")
	case "mostcomplete":
		sess.Desc("completeness")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	default:
		sess.Asc("deadline_unix")
	}
	return miles, sess.Find(&miles)
}

// GetMilestonesByRepoIDs returns a list of milestones of given repositories and status.
func GetMilestonesByRepoIDs(ctx context.Context, repoIDs []int64, page int, isClosed bool, sortType string) (MilestoneList, error) {
	return SearchMilestones(
		ctx,
		builder.In("repo_id", repoIDs),
		page,
		isClosed,
		sortType,
		"",
	)
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

// CountMilestones returns number of milestones in given repository with other options
func CountMilestones(ctx context.Context, opts GetMilestonesOption) (int64, error) {
	return db.GetEngine(ctx).
		Where(opts.toCond()).
		Count(new(Milestone))
}

// CountMilestonesByRepoCond map from repo conditions to number of milestones matching the options`
func CountMilestonesByRepoCond(ctx context.Context, repoCond builder.Cond, isClosed bool) (map[int64]int64, error) {
	sess := db.GetEngine(ctx).Where("is_closed = ?", isClosed)
	if repoCond.IsValid() {
		sess.In("repo_id", builder.Select("id").From("repository").Where(repoCond))
	}

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

// CountMilestonesByRepoCondAndKw map from repo conditions and the keyword of milestones' name to number of milestones matching the options`
func CountMilestonesByRepoCondAndKw(ctx context.Context, repoCond builder.Cond, keyword string, isClosed bool) (map[int64]int64, error) {
	sess := db.GetEngine(ctx).Where("is_closed = ?", isClosed)
	if len(keyword) > 0 {
		sess = sess.And(builder.Like{"UPPER(name)", strings.ToUpper(keyword)})
	}
	if repoCond.IsValid() {
		sess.In("repo_id", builder.Select("id").From("repository").Where(repoCond))
	}

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

// GetMilestonesStatsByRepoCond returns milestone statistic information for dashboard by given conditions.
func GetMilestonesStatsByRepoCond(ctx context.Context, repoCond builder.Cond) (*MilestonesStats, error) {
	var err error
	stats := &MilestonesStats{}

	sess := db.GetEngine(ctx).Where("is_closed = ?", false)
	if repoCond.IsValid() {
		sess.And(builder.In("repo_id", builder.Select("id").From("repository").Where(repoCond)))
	}
	stats.OpenCount, err = sess.Count(new(Milestone))
	if err != nil {
		return nil, err
	}

	sess = db.GetEngine(ctx).Where("is_closed = ?", true)
	if repoCond.IsValid() {
		sess.And(builder.In("repo_id", builder.Select("id").From("repository").Where(repoCond)))
	}
	stats.ClosedCount, err = sess.Count(new(Milestone))
	if err != nil {
		return nil, err
	}

	return stats, nil
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
