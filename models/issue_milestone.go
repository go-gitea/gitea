// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Milestone represents a milestone of repository.
type Milestone struct {
	ID              int64       `xorm:"pk autoincr"`
	RepoID          int64       `xorm:"INDEX"`
	Repo            *Repository `xorm:"-"`
	Name            string
	Content         string `xorm:"TEXT"`
	RenderedContent string `xorm:"-"`
	IsClosed        bool
	NumIssues       int
	NumClosedIssues int
	NumOpenIssues   int  `xorm:"-"`
	Completeness    int  // Percentage(1-100).
	IsOverdue       bool `xorm:"-"`

	DeadlineString string `xorm:"-"`
	DeadlineUnix   timeutil.TimeStamp
	ClosedDateUnix timeutil.TimeStamp

	TotalTrackedTime int64 `xorm:"-"`
}

// BeforeUpdate is invoked from XORM before updating this object.
func (m *Milestone) BeforeUpdate() {
	if m.NumIssues > 0 {
		m.Completeness = m.NumClosedIssues * 100 / m.NumIssues
	} else {
		m.Completeness = 0
	}
}

// AfterLoad is invoked from XORM after setting the value of a field of
// this object.
func (m *Milestone) AfterLoad() {
	m.NumOpenIssues = m.NumIssues - m.NumClosedIssues
	if m.DeadlineUnix.Year() == 9999 {
		return
	}

	m.DeadlineString = m.DeadlineUnix.Format("2006-01-02")
	if timeutil.TimeStampNow() >= m.DeadlineUnix {
		m.IsOverdue = true
	}
}

// State returns string representation of milestone status.
func (m *Milestone) State() api.StateType {
	if m.IsClosed {
		return api.StateClosed
	}
	return api.StateOpen
}

// APIFormat returns this Milestone in API format.
func (m *Milestone) APIFormat() *api.Milestone {
	apiMilestone := &api.Milestone{
		ID:           m.ID,
		State:        m.State(),
		Title:        m.Name,
		Description:  m.Content,
		OpenIssues:   m.NumOpenIssues,
		ClosedIssues: m.NumClosedIssues,
	}
	if m.IsClosed {
		apiMilestone.Closed = m.ClosedDateUnix.AsTimePtr()
	}
	if m.DeadlineUnix.Year() < 9999 {
		apiMilestone.Deadline = m.DeadlineUnix.AsTimePtr()
	}
	return apiMilestone
}

// NewMilestone creates new milestone of repository.
func NewMilestone(m *Milestone) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	m.Name = strings.TrimSpace(m.Name)

	if _, err = sess.Insert(m); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `repository` SET num_milestones = num_milestones + 1 WHERE id = ?", m.RepoID); err != nil {
		return err
	}
	return sess.Commit()
}

func getMilestoneByRepoID(e Engine, repoID, id int64) (*Milestone, error) {
	m := &Milestone{
		ID:     id,
		RepoID: repoID,
	}
	has, err := e.Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrMilestoneNotExist{id, repoID}
	}
	return m, nil
}

// GetMilestoneByRepoID returns the milestone in a repository.
func GetMilestoneByRepoID(repoID, id int64) (*Milestone, error) {
	return getMilestoneByRepoID(x, repoID, id)
}

// GetMilestoneByID returns the milestone via id .
func GetMilestoneByID(id int64) (*Milestone, error) {
	var m Milestone
	has, err := x.ID(id).Get(&m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrMilestoneNotExist{id, 0}
	}
	return &m, nil
}

// MilestoneList is a list of milestones offering additional functionality
type MilestoneList []*Milestone

func (milestones MilestoneList) loadTotalTrackedTimes(e Engine) error {
	type totalTimesByMilestone struct {
		MilestoneID int64
		Time        int64
	}
	if len(milestones) == 0 {
		return nil
	}
	var trackedTimes = make(map[int64]int64, len(milestones))

	// Get total tracked time by milestone_id
	rows, err := e.Table("issue").
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

func (m *Milestone) loadTotalTrackedTime(e Engine) error {
	type totalTimesByMilestone struct {
		MilestoneID int64
		Time        int64
	}
	totalTime := &totalTimesByMilestone{MilestoneID: m.ID}
	has, err := e.Table("issue").
		Join("INNER", "milestone", "issue.milestone_id = milestone.id").
		Join("LEFT", "tracked_time", "tracked_time.issue_id = issue.id").
		Where("tracked_time.deleted = ?", false).
		Select("milestone_id, sum(time) as time").
		Where("milestone_id = ?", m.ID).
		GroupBy("milestone_id").
		Get(totalTime)
	if err != nil {
		return err
	} else if !has {
		return nil
	}
	m.TotalTrackedTime = totalTime.Time
	return nil
}

// LoadTotalTrackedTimes loads for every milestone in the list the TotalTrackedTime by a batch request
func (milestones MilestoneList) LoadTotalTrackedTimes() error {
	return milestones.loadTotalTrackedTimes(x)
}

// LoadTotalTrackedTime loads the tracked time for the milestone
func (m *Milestone) LoadTotalTrackedTime() error {
	return m.loadTotalTrackedTime(x)
}

func (milestones MilestoneList) getMilestoneIDs() []int64 {
	var ids = make([]int64, 0, len(milestones))
	for _, ms := range milestones {
		ids = append(ids, ms.ID)
	}
	return ids
}

// GetMilestonesByRepoID returns all opened milestones of a repository.
func GetMilestonesByRepoID(repoID int64, state api.StateType) (MilestoneList, error) {
	sess := x.Where("repo_id = ?", repoID)

	switch state {
	case api.StateClosed:
		sess = sess.And("is_closed = ?", true)

	case api.StateAll:
		break

	case api.StateOpen:
		fallthrough

	default:
		sess = sess.And("is_closed = ?", false)
	}

	miles := make([]*Milestone, 0, 10)
	return miles, sess.Asc("deadline_unix").Asc("id").Find(&miles)
}

// GetMilestones returns a list of milestones of given repository and status.
func GetMilestones(repoID int64, page int, isClosed bool, sortType string) (MilestoneList, error) {
	miles := make([]*Milestone, 0, setting.UI.IssuePagingNum)
	sess := x.Where("repo_id = ? AND is_closed = ?", repoID, isClosed)
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

func updateMilestone(e Engine, m *Milestone) error {
	m.Name = strings.TrimSpace(m.Name)
	_, err := e.ID(m.ID).AllCols().
		SetExpr("num_issues", builder.Select("count(*)").From("issue").Where(
			builder.Eq{"milestone_id": m.ID},
		)).
		SetExpr("num_closed_issues", builder.Select("count(*)").From("issue").Where(
			builder.Eq{
				"milestone_id": m.ID,
				"is_closed":    true,
			},
		)).
		Update(m)
	return err
}

// UpdateMilestone updates information of given milestone.
func UpdateMilestone(m *Milestone, oldIsClosed bool) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if m.IsClosed && !oldIsClosed {
		m.ClosedDateUnix = timeutil.TimeStampNow()
	}

	if err := updateMilestone(sess, m); err != nil {
		return err
	}

	if err := updateMilestoneCompleteness(sess, m.ID); err != nil {
		return err
	}

	// if IsClosed changed, update milestone numbers of repository
	if oldIsClosed != m.IsClosed {
		if err := updateRepoMilestoneNum(sess, m.RepoID); err != nil {
			return err
		}
	}

	return sess.Commit()
}

func updateMilestoneCompleteness(e Engine, milestoneID int64) error {
	_, err := e.Exec("UPDATE `milestone` SET completeness=100*num_closed_issues/(CASE WHEN num_issues > 0 THEN num_issues ELSE 1 END) WHERE id=?",
		milestoneID,
	)
	return err
}

func countRepoMilestones(e Engine, repoID int64) (int64, error) {
	return e.
		Where("repo_id=?", repoID).
		Count(new(Milestone))
}

func countRepoClosedMilestones(e Engine, repoID int64) (int64, error) {
	return e.
		Where("repo_id=? AND is_closed=?", repoID, true).
		Count(new(Milestone))
}

// CountRepoClosedMilestones returns number of closed milestones in given repository.
func CountRepoClosedMilestones(repoID int64) (int64, error) {
	return countRepoClosedMilestones(x, repoID)
}

// MilestoneStats returns number of open and closed milestones of given repository.
func MilestoneStats(repoID int64) (open int64, closed int64, err error) {
	open, err = x.
		Where("repo_id=? AND is_closed=?", repoID, false).
		Count(new(Milestone))
	if err != nil {
		return 0, 0, nil
	}
	closed, err = CountRepoClosedMilestones(repoID)
	return open, closed, err
}

// ChangeMilestoneStatus changes the milestone open/closed status.
func ChangeMilestoneStatus(m *Milestone, isClosed bool) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	m.IsClosed = isClosed
	if isClosed {
		m.ClosedDateUnix = timeutil.TimeStampNow()
	}

	if _, err := sess.ID(m.ID).Cols("is_closed", "closed_date_unix").Update(m); err != nil {
		return err
	}

	if err := updateRepoMilestoneNum(sess, m.RepoID); err != nil {
		return err
	}

	return sess.Commit()
}

func updateRepoMilestoneNum(e Engine, repoID int64) error {
	_, err := e.Exec("UPDATE `repository` SET num_milestones=(SELECT count(*) FROM milestone WHERE repo_id=?),num_closed_milestones=(SELECT count(*) FROM milestone WHERE repo_id=? AND is_closed=?) WHERE id=?",
		repoID,
		repoID,
		true,
		repoID,
	)
	return err
}

func updateMilestoneTotalNum(e Engine, milestoneID int64) (err error) {
	if _, err = e.Exec("UPDATE `milestone` SET num_issues=(SELECT count(*) FROM issue WHERE milestone_id=?) WHERE id=?",
		milestoneID,
		milestoneID,
	); err != nil {
		return
	}

	return updateMilestoneCompleteness(e, milestoneID)
}

func updateMilestoneClosedNum(e Engine, milestoneID int64) (err error) {
	if _, err = e.Exec("UPDATE `milestone` SET num_closed_issues=(SELECT count(*) FROM issue WHERE milestone_id=? AND is_closed=?) WHERE id=?",
		milestoneID,
		true,
		milestoneID,
	); err != nil {
		return
	}

	return updateMilestoneCompleteness(e, milestoneID)
}

func changeMilestoneAssign(e *xorm.Session, doer *User, issue *Issue, oldMilestoneID int64) error {
	if err := updateIssueCols(e, issue, "milestone_id"); err != nil {
		return err
	}

	if oldMilestoneID > 0 {
		if err := updateMilestoneTotalNum(e, oldMilestoneID); err != nil {
			return err
		}
		if issue.IsClosed {
			if err := updateMilestoneClosedNum(e, oldMilestoneID); err != nil {
				return err
			}
		}
	}

	if issue.MilestoneID > 0 {
		if err := updateMilestoneTotalNum(e, issue.MilestoneID); err != nil {
			return err
		}
		if issue.IsClosed {
			if err := updateMilestoneClosedNum(e, issue.MilestoneID); err != nil {
				return err
			}
		}
	}

	if oldMilestoneID > 0 || issue.MilestoneID > 0 {
		if err := issue.loadRepo(e); err != nil {
			return err
		}

		var opts = &CreateCommentOptions{
			Type:           CommentTypeMilestone,
			Doer:           doer,
			Repo:           issue.Repo,
			Issue:          issue,
			OldMilestoneID: oldMilestoneID,
			MilestoneID:    issue.MilestoneID,
		}
		if _, err := createComment(e, opts); err != nil {
			return err
		}
	}

	return nil
}

// ChangeMilestoneAssign changes assignment of milestone for issue.
func ChangeMilestoneAssign(issue *Issue, doer *User, oldMilestoneID int64) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = changeMilestoneAssign(sess, doer, issue, oldMilestoneID); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}
	return nil
}

// DeleteMilestoneByRepoID deletes a milestone from a repository.
func DeleteMilestoneByRepoID(repoID, id int64) error {
	m, err := GetMilestoneByRepoID(repoID, id)
	if err != nil {
		if IsErrMilestoneNotExist(err) {
			return nil
		}
		return err
	}

	repo, err := GetRepositoryByID(m.RepoID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.ID(m.ID).Delete(new(Milestone)); err != nil {
		return err
	}

	numMilestones, err := countRepoMilestones(sess, repo.ID)
	if err != nil {
		return err
	}
	numClosedMilestones, err := countRepoClosedMilestones(sess, repo.ID)
	if err != nil {
		return err
	}
	repo.NumMilestones = int(numMilestones)
	repo.NumClosedMilestones = int(numClosedMilestones)

	if _, err = sess.ID(repo.ID).Cols("num_milestones, num_closed_milestones").Update(repo); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `issue` SET milestone_id = 0 WHERE milestone_id = ?", m.ID); err != nil {
		return err
	}
	return sess.Commit()
}

// CountMilestonesByRepoIDs map from repoIDs to number of milestones matching the options`
func CountMilestonesByRepoIDs(repoIDs []int64, isClosed bool) (map[int64]int64, error) {
	sess := x.Where("is_closed = ?", isClosed)
	sess.In("repo_id", repoIDs)

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

// GetMilestonesByRepoIDs returns a list of milestones of given repositories and status.
func GetMilestonesByRepoIDs(repoIDs []int64, page int, isClosed bool, sortType string) (MilestoneList, error) {
	miles := make([]*Milestone, 0, setting.UI.IssuePagingNum)
	sess := x.Where("is_closed = ?", isClosed)
	sess.In("repo_id", repoIDs)
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

// MilestonesStats represents milestone statistic information.
type MilestonesStats struct {
	OpenCount, ClosedCount int64
}

// GetMilestonesStats returns milestone statistic information for dashboard by given conditions.
func GetMilestonesStats(userRepoIDs []int64) (*MilestonesStats, error) {
	var err error
	stats := &MilestonesStats{}

	stats.OpenCount, err = x.Where("is_closed = ?", false).
		And(builder.In("repo_id", userRepoIDs)).
		Count(new(Milestone))
	if err != nil {
		return nil, err
	}
	stats.ClosedCount, err = x.Where("is_closed = ?", true).
		And(builder.In("repo_id", userRepoIDs)).
		Count(new(Milestone))
	if err != nil {
		return nil, err
	}

	return stats, nil
}
