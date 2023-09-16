// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrMilestoneNotExist represents a "MilestoneNotExist" kind of error.
type ErrMilestoneNotExist struct {
	ID     int64
	RepoID int64
	Name   string
}

// IsErrMilestoneNotExist checks if an error is a ErrMilestoneNotExist.
func IsErrMilestoneNotExist(err error) bool {
	_, ok := err.(ErrMilestoneNotExist)
	return ok
}

func (err ErrMilestoneNotExist) Error() string {
	if len(err.Name) > 0 {
		return fmt.Sprintf("milestone does not exist [name: %s, repo_id: %d]", err.Name, err.RepoID)
	}
	return fmt.Sprintf("milestone does not exist [id: %d, repo_id: %d]", err.ID, err.RepoID)
}

func (err ErrMilestoneNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Milestone represents a milestone of repository.
type Milestone struct {
	ID              int64                  `xorm:"pk autoincr"`
	RepoID          int64                  `xorm:"INDEX"`
	Repo            *repo_model.Repository `xorm:"-"`
	Name            string
	Content         string `xorm:"TEXT"`
	RenderedContent string `xorm:"-"`
	IsClosed        bool
	NumIssues       int
	NumClosedIssues int
	NumOpenIssues   int  `xorm:"-"`
	Completeness    int  // Percentage(1-100).
	IsOverdue       bool `xorm:"-"`

	CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	DeadlineUnix   timeutil.TimeStamp
	ClosedDateUnix timeutil.TimeStamp
	DeadlineString string `xorm:"-"`

	TotalTrackedTime int64 `xorm:"-"`
}

func init() {
	db.RegisterModel(new(Milestone))
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
	if m.IsClosed {
		m.IsOverdue = m.ClosedDateUnix >= m.DeadlineUnix
	} else {
		m.IsOverdue = timeutil.TimeStampNow() >= m.DeadlineUnix
	}
}

// State returns string representation of milestone status.
func (m *Milestone) State() api.StateType {
	if m.IsClosed {
		return api.StateClosed
	}
	return api.StateOpen
}

// NewMilestone creates new milestone of repository.
func NewMilestone(ctx context.Context, m *Milestone) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	m.Name = strings.TrimSpace(m.Name)

	if err = db.Insert(ctx, m); err != nil {
		return err
	}

	if _, err = db.Exec(ctx, "UPDATE `repository` SET num_milestones = num_milestones + 1 WHERE id = ?", m.RepoID); err != nil {
		return err
	}
	return committer.Commit()
}

// HasMilestoneByRepoID returns if the milestone exists in the repository.
func HasMilestoneByRepoID(ctx context.Context, repoID, id int64) (bool, error) {
	return db.GetEngine(ctx).ID(id).Where("repo_id=?", repoID).Exist(new(Milestone))
}

// GetMilestoneByRepoID returns the milestone in a repository.
func GetMilestoneByRepoID(ctx context.Context, repoID, id int64) (*Milestone, error) {
	m := new(Milestone)
	has, err := db.GetEngine(ctx).ID(id).Where("repo_id=?", repoID).Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrMilestoneNotExist{ID: id, RepoID: repoID}
	}
	return m, nil
}

// GetMilestoneByRepoIDANDName return a milestone if one exist by name and repo
func GetMilestoneByRepoIDANDName(ctx context.Context, repoID int64, name string) (*Milestone, error) {
	var mile Milestone
	has, err := db.GetEngine(ctx).Where("repo_id=? AND name=?", repoID, name).Get(&mile)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrMilestoneNotExist{Name: name, RepoID: repoID}
	}
	return &mile, nil
}

// UpdateMilestone updates information of given milestone.
func UpdateMilestone(ctx context.Context, m *Milestone, oldIsClosed bool) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if m.IsClosed && !oldIsClosed {
		m.ClosedDateUnix = timeutil.TimeStampNow()
	}

	if err := updateMilestone(ctx, m); err != nil {
		return err
	}

	// if IsClosed changed, update milestone numbers of repository
	if oldIsClosed != m.IsClosed {
		if err := updateRepoMilestoneNum(ctx, m.RepoID); err != nil {
			return err
		}
	}

	return committer.Commit()
}

func updateMilestone(ctx context.Context, m *Milestone) error {
	m.Name = strings.TrimSpace(m.Name)
	_, err := db.GetEngine(ctx).ID(m.ID).AllCols().Update(m)
	if err != nil {
		return err
	}
	return UpdateMilestoneCounters(ctx, m.ID)
}

// UpdateMilestoneCounters calculates NumIssues, NumClosesIssues and Completeness
func UpdateMilestoneCounters(ctx context.Context, id int64) error {
	e := db.GetEngine(ctx)
	_, err := e.ID(id).
		SetExpr("num_issues", builder.Select("count(*)").From("issue").Where(
			builder.Eq{"milestone_id": id},
		)).
		SetExpr("num_closed_issues", builder.Select("count(*)").From("issue").Where(
			builder.Eq{
				"milestone_id": id,
				"is_closed":    true,
			},
		)).
		Update(&Milestone{})
	if err != nil {
		return err
	}
	_, err = e.Exec("UPDATE `milestone` SET completeness=100*num_closed_issues/(CASE WHEN num_issues > 0 THEN num_issues ELSE 1 END) WHERE id=?",
		id,
	)
	return err
}

// ChangeMilestoneStatusByRepoIDAndID changes a milestone open/closed status if the milestone ID is in the repo.
func ChangeMilestoneStatusByRepoIDAndID(ctx context.Context, repoID, milestoneID int64, isClosed bool) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	m := &Milestone{
		ID:     milestoneID,
		RepoID: repoID,
	}

	has, err := db.GetEngine(ctx).ID(milestoneID).Where("repo_id = ?", repoID).Get(m)
	if err != nil {
		return err
	} else if !has {
		return ErrMilestoneNotExist{ID: milestoneID, RepoID: repoID}
	}

	if err := changeMilestoneStatus(ctx, m, isClosed); err != nil {
		return err
	}

	return committer.Commit()
}

// ChangeMilestoneStatus changes the milestone open/closed status.
func ChangeMilestoneStatus(ctx context.Context, m *Milestone, isClosed bool) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := changeMilestoneStatus(ctx, m, isClosed); err != nil {
		return err
	}

	return committer.Commit()
}

func changeMilestoneStatus(ctx context.Context, m *Milestone, isClosed bool) error {
	m.IsClosed = isClosed
	if isClosed {
		m.ClosedDateUnix = timeutil.TimeStampNow()
	}

	count, err := db.GetEngine(ctx).ID(m.ID).Where("repo_id = ? AND is_closed = ?", m.RepoID, !isClosed).Cols("is_closed", "closed_date_unix").Update(m)
	if err != nil {
		return err
	}
	if count < 1 {
		return nil
	}
	return updateRepoMilestoneNum(ctx, m.RepoID)
}

// DeleteMilestoneByRepoID deletes a milestone from a repository.
func DeleteMilestoneByRepoID(ctx context.Context, repoID, id int64) error {
	m, err := GetMilestoneByRepoID(ctx, repoID, id)
	if err != nil {
		if IsErrMilestoneNotExist(err) {
			return nil
		}
		return err
	}

	repo, err := repo_model.GetRepositoryByID(ctx, m.RepoID)
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	if _, err = sess.ID(m.ID).Delete(new(Milestone)); err != nil {
		return err
	}

	numMilestones, err := CountMilestones(ctx, GetMilestonesOption{
		RepoID: repo.ID,
		State:  api.StateAll,
	})
	if err != nil {
		return err
	}
	numClosedMilestones, err := CountMilestones(ctx, GetMilestonesOption{
		RepoID: repo.ID,
		State:  api.StateClosed,
	})
	if err != nil {
		return err
	}
	repo.NumMilestones = int(numMilestones)
	repo.NumClosedMilestones = int(numClosedMilestones)

	if _, err = sess.ID(repo.ID).Cols("num_milestones, num_closed_milestones").Update(repo); err != nil {
		return err
	}

	if _, err = db.Exec(ctx, "UPDATE `issue` SET milestone_id = 0 WHERE milestone_id = ?", m.ID); err != nil {
		return err
	}
	return committer.Commit()
}

func updateRepoMilestoneNum(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `repository` SET num_milestones=(SELECT count(*) FROM milestone WHERE repo_id=?),num_closed_milestones=(SELECT count(*) FROM milestone WHERE repo_id=? AND is_closed=?) WHERE id=?",
		repoID,
		repoID,
		true,
		repoID,
	)
	return err
}

// LoadTotalTrackedTime loads the tracked time for the milestone
func (m *Milestone) LoadTotalTrackedTime(ctx context.Context) error {
	type totalTimesByMilestone struct {
		MilestoneID int64
		Time        int64
	}
	totalTime := &totalTimesByMilestone{MilestoneID: m.ID}
	has, err := db.GetEngine(ctx).Table("issue").
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

// InsertMilestones creates milestones of repository.
func InsertMilestones(ctx context.Context, ms ...*Milestone) (err error) {
	if len(ms) == 0 {
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	// to return the id, so we should not use batch insert
	for _, m := range ms {
		if _, err = sess.NoAutoTime().Insert(m); err != nil {
			return err
		}
	}

	if _, err = db.Exec(ctx, "UPDATE `repository` SET num_milestones = num_milestones + ? WHERE id = ?", len(ms), ms[0].RepoID); err != nil {
		return err
	}
	return committer.Commit()
}
