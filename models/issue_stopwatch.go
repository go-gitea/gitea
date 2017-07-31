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
func CreateOrStopIssueStopwatch(userID int64, issueID int64) error {
	sw, exists, err := getStopwatch(x, userID, issueID)
	if err != nil {
		return err
	}
	if exists {
		// Create tracked time out of the time difference between start date and actual date
		timediff := time.Now().Unix() - sw.CreatedUnix

		// Create TrackedTime
		tt := &TrackedTime{
			Created: time.Now(),
			IssueID: issueID,
			UserID:  userID,
			Time:    timediff,
		}

		if _, err := x.Insert(tt); err != nil {
			return err
		}
		// Add comment referencing to the tracked time
		comment := &Comment{
			IssueID:  issueID,
			PosterID: userID,
			Type:     CommentTypeStopTracking,
			Content:  secToTime(timediff),
		}

		if _, err := x.Insert(comment); err != nil {
			return err
		}

		if _, err := x.Delete(sw); err != nil {
			return err
		}
	} else {
		// Create stopwatch
		sw = &Stopwatch{
			UserID:  userID,
			IssueID: issueID,
			Created: time.Now(),
		}

		if _, err := x.Insert(sw); err != nil {
			return err
		}

		// Add comment referencing to the stopwatch
		comment := &Comment{
			IssueID:  issueID,
			PosterID: userID,
			Type:     CommentTypeStartTracking,
		}

		if _, err := x.Insert(comment); err != nil {
			return err
		}
	}
	return nil
}

// CancelStopwatch removes the given stopwatch and logs it into issue's timeline.
func CancelStopwatch(userID int64, issueID int64) error {
	sw, exists, err := getStopwatch(x, userID, issueID)
	if err != nil {
		return err
	}

	if exists {
		if _, err := x.Delete(sw); err != nil {
			return err
		}
		comment := &Comment{
			PosterID: userID,
			IssueID:  issueID,
			Type:     CommentTypeCancelTracking,
		}

		if _, err := x.Insert(comment); err != nil {
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

// BeforeInsert will be invoked by XORM before inserting a record
// representing this object.
func (s *Stopwatch) BeforeInsert() {
	s.CreatedUnix = time.Now().Unix()
}
