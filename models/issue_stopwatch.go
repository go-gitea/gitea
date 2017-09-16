// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"github.com/go-xorm/xorm"
)

// Stopwatch represents a stopwatch for time tracking.
type Stopwatch struct {
	ID          int64     `xorm:"pk autoincr"`
	IssueID     int64     `xorm:"INDEX"`
	UserID      int64     `xorm:"INDEX"`
	Created     time.Time `xorm:"-"`
	CreatedUnix int64
}

// BeforeInsert will be invoked by XORM before inserting a record
// representing this object.
func (s *Stopwatch) BeforeInsert() {
	s.CreatedUnix = time.Now().Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (s *Stopwatch) AfterSet(colName string, _ xorm.Cell) {
	switch colName {

	case "created_unix":
		s.Created = time.Unix(s.CreatedUnix, 0).Local()
	}
}

func getStopwatch(e Engine, userID, issueID int64) (sw *Stopwatch, exists bool, err error) {
	sw = new(Stopwatch)
	exists, err = e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(sw)
	return
}

// StopwatchExists returns true if the stopwatch exists
func StopwatchExists(userID int64, issueID int64) bool {
	_, exists, _ := getStopwatch(x, userID, issueID)
	return exists
}

// HasUserStopwatch returns true if the user has a stopwatch
func HasUserStopwatch(userID int64) (exists bool, sw *Stopwatch, err error) {
	sw = new(Stopwatch)
	exists, err = x.
		Where("user_id = ?", userID).
		Get(sw)
	return
}

// CreateOrStopIssueStopwatch will create or remove a stopwatch and will log it into issue's timeline.
func CreateOrStopIssueStopwatch(user *User, issue *Issue) error {
	sw, exists, err := getStopwatch(x, user.ID, issue.ID)
	if err != nil {
		return err
	}
	if exists {
		// Create tracked time out of the time difference between start date and actual date
		timediff := time.Now().Unix() - sw.CreatedUnix

		// Create TrackedTime
		tt := &TrackedTime{
			Created: time.Now(),
			IssueID: issue.ID,
			UserID:  user.ID,
			Time:    timediff,
		}

		if _, err := x.Insert(tt); err != nil {
			return err
		}

		if _, err := CreateComment(&CreateCommentOptions{
			Doer:    user,
			Issue:   issue,
			Repo:    issue.Repo,
			Content: secToTime(timediff),
			Type:    CommentTypeStopTracking,
		}); err != nil {
			return err
		}
		if _, err := x.Delete(sw); err != nil {
			return err
		}
	} else {
		// Create stopwatch
		sw = &Stopwatch{
			UserID:  user.ID,
			IssueID: issue.ID,
			Created: time.Now(),
		}

		if _, err := x.Insert(sw); err != nil {
			return err
		}

		if _, err := CreateComment(&CreateCommentOptions{
			Doer:  user,
			Issue: issue,
			Repo:  issue.Repo,
			Type:  CommentTypeStartTracking,
		}); err != nil {
			return err
		}
	}
	return nil
}

// CancelStopwatch removes the given stopwatch and logs it into issue's timeline.
func CancelStopwatch(user *User, issue *Issue) error {
	sw, exists, err := getStopwatch(x, user.ID, issue.ID)
	if err != nil {
		return err
	}

	if exists {
		if _, err := x.Delete(sw); err != nil {
			return err
		}

		if _, err := CreateComment(&CreateCommentOptions{
			Doer:  user,
			Issue: issue,
			Repo:  issue.Repo,
			Type:  CommentTypeCancelTracking,
		}); err != nil {
			return err
		}
	}
	return nil
}

func secToTime(duration int64) string {
	seconds := duration % 60
	minutes := (duration / (60)) % 60
	hours := duration / (60 * 60)

	var hrs string

	if hours > 0 {
		hrs = fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		if hours == 0 {
			hrs = fmt.Sprintf("%dmin", minutes)
		} else {
			hrs = fmt.Sprintf("%s %dmin", hrs, minutes)
		}
	}
	if seconds > 0 {
		if hours == 0 && minutes == 0 {
			hrs = fmt.Sprintf("%ds", seconds)
		} else {
			hrs = fmt.Sprintf("%s %ds", hrs, seconds)
		}
	}

	return hrs
}
