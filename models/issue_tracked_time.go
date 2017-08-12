// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	"github.com/go-xorm/builder"
	"github.com/go-xorm/xorm"
)

// TrackedTime represents a time that was spent for a specific issue.
type TrackedTime struct {
	ID          int64     `xorm:"pk autoincr" json:"id"`
	IssueID     int64     `xorm:"INDEX" json:"issue_id"`
	UserID      int64     `xorm:"INDEX" json:"user_id"`
	Created     time.Time `xorm:"-" json:"created"`
	CreatedUnix int64     `json:"-"`
	Time        int64     `json:"time"`
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (t *TrackedTime) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		t.Created = time.Unix(t.CreatedUnix, 0).Local()
	}
}

// FindTrackedTimesOptions represent the filters for tracked times. If an ID is 0 it will be ignored.
type FindTrackedTimesOptions struct {
	IssueID      int64
	UserID       int64
	RepositoryID int64
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
	return cond
}

// GetTrackedTimes returns all tracked times that fit to the given options.
func GetTrackedTimes(options FindTrackedTimesOptions) (trackedTimes []*TrackedTime, err error) {
	if options.RepositoryID > 0 {
		err = x.Join("INNER", "issue", "issue.id = tracked_time.issue_id").Where(options.ToCond()).Find(&trackedTimes)
		return
	}
	err = x.Where(options.ToCond()).Find(&trackedTimes)
	return
}

// BeforeInsert will be invoked by XORM before inserting a record
// representing this object.
func (t *TrackedTime) BeforeInsert() {
	t.CreatedUnix = time.Now().Unix()
}

// AddTime will add the given time (in seconds) to the issue
func AddTime(userID int64, issueID int64, time int64) (*TrackedTime, error) {
	tt := &TrackedTime{
		IssueID: issueID,
		UserID:  userID,
		Time:    time,
	}
	if _, err := x.Insert(tt); err != nil {
		return nil, err
	}
	comment := &Comment{
		IssueID:  issueID,
		PosterID: userID,
		Type:     CommentTypeAddTimeManual,
		Content:  secToTime(time),
	}
	if _, err := x.Insert(comment); err != nil {
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
		if total, ok := totalTimesByUser[t.UserID]; !ok {
			totalTimesByUser[t.UserID] = t.Time
		} else {
			totalTimesByUser[t.UserID] = total + t.Time
		}
	}

	totalTimes := make(map[*User]string)
	//Fetching User and making time human readable
	for userID, total := range totalTimesByUser {
		user, err := GetUserByID(userID)
		if err != nil || user == nil {
			continue
		}
		totalTimes[user] = secToTime(total)
	}
	return totalTimes, nil
}
