// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/go-xorm/xorm"
	"xorm.io/builder"
)

// TrackedTime represents a time that was spent for a specific issue.
type TrackedTime struct {
	ID          int64     `xorm:"pk autoincr" json:"id"`
	IssueID     int64     `xorm:"INDEX" json:"issue_id"`
	UserID      int64     `xorm:"INDEX" json:"user_id"`
	Created     time.Time `xorm:"-" json:"created"`
	CreatedUnix int64     `xorm:"created" json:"-"`
	Time        int64     `json:"time"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (t *TrackedTime) AfterLoad() {
	t.Created = time.Unix(t.CreatedUnix, 0).In(setting.DefaultUILocation)
}

// APIFormat converts TrackedTime to API format
func (t *TrackedTime) APIFormat() *api.TrackedTime {
	return &api.TrackedTime{
		ID:      t.ID,
		IssueID: t.IssueID,
		UserID:  t.UserID,
		Time:    t.Time,
		Created: t.Created,
	}
}

// FindTrackedTimesOptions represent the filters for tracked times. If an ID is 0 it will be ignored.
type FindTrackedTimesOptions struct {
	IssueID      int64
	UserID       int64
	RepositoryID int64
	MilestoneID  int64
}

// ToCond will convert each condition into a xorm-Cond
func (opts *FindTrackedTimesOptions) ToCond() builder.Cond {
	cond := builder.NewCond()
	if opts.IssueID != 0 {
		cond = cond.And(builder.Eq{"issue_id": opts.IssueID})
	}
	if opts.UserID != 0 {
		cond = cond.And(builder.Eq{"user_id": opts.UserID})
	}
	if opts.RepositoryID != 0 {
		cond = cond.And(builder.Eq{"issue.repo_id": opts.RepositoryID})
	}
	if opts.MilestoneID != 0 {
		cond = cond.And(builder.Eq{"issue.milestone_id": opts.MilestoneID})
	}
	return cond
}

// ToSession will convert the given options to a xorm Session by using the conditions from ToCond and joining with issue table if required
func (opts *FindTrackedTimesOptions) ToSession(e Engine) *xorm.Session {
	if opts.RepositoryID > 0 || opts.MilestoneID > 0 {
		return e.Join("INNER", "issue", "issue.id = tracked_time.issue_id").Where(opts.ToCond())
	}
	return x.Where(opts.ToCond())
}

// GetTrackedTimes returns all tracked times that fit to the given options.
func GetTrackedTimes(options FindTrackedTimesOptions) (trackedTimes []*TrackedTime, err error) {
	err = options.ToSession(x).Find(&trackedTimes)
	return
}

// AddTime will add the given time (in seconds) to the issue
func AddTime(user *User, issue *Issue, time int64) (*TrackedTime, error) {
	tt := &TrackedTime{
		IssueID: issue.ID,
		UserID:  user.ID,
		Time:    time,
	}
	if _, err := x.Insert(tt); err != nil {
		return nil, err
	}
	if err := issue.loadRepo(x); err != nil {
		return nil, err
	}
	if _, err := CreateComment(&CreateCommentOptions{
		Issue:   issue,
		Repo:    issue.Repo,
		Doer:    user,
		Content: SecToTime(time),
		Type:    CommentTypeAddTimeManual,
	}); err != nil {
		return nil, err
	}
	return tt, nil
}

// TotalTimes returns the spent time for each user by an issue
func TotalTimes(options FindTrackedTimesOptions) (map[*User]string, error) {
	trackedTimes, err := GetTrackedTimes(options)
	if err != nil {
		return nil, err
	}
	//Adding total time per user ID
	totalTimesByUser := make(map[int64]int64)
	for _, t := range trackedTimes {
		totalTimesByUser[t.UserID] += t.Time
	}

	totalTimes := make(map[*User]string)
	//Fetching User and making time human readable
	for userID, total := range totalTimesByUser {
		user, err := GetUserByID(userID)
		if err != nil {
			if IsErrUserNotExist(err) {
				continue
			}
			return nil, err
		}
		totalTimes[user] = SecToTime(total)
	}
	return totalTimes, nil
}
