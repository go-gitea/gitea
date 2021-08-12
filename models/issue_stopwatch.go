// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// Stopwatch represents a stopwatch for time tracking.
type Stopwatch struct {
	ID          int64              `xorm:"pk autoincr"`
	IssueID     int64              `xorm:"INDEX"`
	UserID      int64              `xorm:"INDEX"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// Seconds returns the amount of time passed since creation, based on local server time
func (s Stopwatch) Seconds() int64 {
	return int64(timeutil.TimeStampNow() - s.CreatedUnix)
}

// Duration returns a human-readable duration string based on local server time
func (s Stopwatch) Duration() string {
	return SecToTime(s.Seconds())
}

func getStopwatch(e Engine, userID, issueID int64) (sw *Stopwatch, exists bool, err error) {
	sw = new(Stopwatch)
	exists, err = e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(sw)
	return
}

// GetUserStopwatches return list of all stopwatches of a user
func GetUserStopwatches(userID int64, listOptions ListOptions) ([]*Stopwatch, error) {
	sws := make([]*Stopwatch, 0, 8)
	sess := x.Where("stopwatch.user_id = ?", userID)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)
	}

	err := sess.Find(&sws)
	if err != nil {
		return nil, err
	}
	return sws, nil
}

// StopwatchExists returns true if the stopwatch exists
func StopwatchExists(userID, issueID int64) bool {
	_, exists, _ := getStopwatch(x, userID, issueID)
	return exists
}

// HasUserStopwatch returns true if the user has a stopwatch
func HasUserStopwatch(userID int64) (exists bool, sw *Stopwatch, err error) {
	return hasUserStopwatch(x, userID)
}

func hasUserStopwatch(e Engine, userID int64) (exists bool, sw *Stopwatch, err error) {
	sw = new(Stopwatch)
	exists, err = e.
		Where("user_id = ?", userID).
		Get(sw)
	return
}

// CreateOrStopIssueStopwatch will create or remove a stopwatch and will log it into issue's timeline.
func CreateOrStopIssueStopwatch(user *User, issue *Issue) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := createOrStopIssueStopwatch(sess, user, issue); err != nil {
		return err
	}
	return sess.Commit()
}

func createOrStopIssueStopwatch(e *xorm.Session, user *User, issue *Issue) error {
	sw, exists, err := getStopwatch(e, user.ID, issue.ID)
	if err != nil {
		return err
	}
	if err := issue.loadRepo(e); err != nil {
		return err
	}

	if exists {
		// Create tracked time out of the time difference between start date and actual date
		timediff := time.Now().Unix() - int64(sw.CreatedUnix)

		// Create TrackedTime
		tt := &TrackedTime{
			Created: time.Now(),
			IssueID: issue.ID,
			UserID:  user.ID,
			Time:    timediff,
		}

		if _, err := e.Insert(tt); err != nil {
			return err
		}

		if _, err := createComment(e, &CreateCommentOptions{
			Doer:    user,
			Issue:   issue,
			Repo:    issue.Repo,
			Content: SecToTime(timediff),
			Type:    CommentTypeStopTracking,
			TimeID:  tt.ID,
		}); err != nil {
			return err
		}
		if _, err := e.Delete(sw); err != nil {
			return err
		}
	} else {
		// if another stopwatch is running: stop it
		exists, sw, err := hasUserStopwatch(e, user.ID)
		if err != nil {
			return err
		}
		if exists {
			issue, err := getIssueByID(e, sw.IssueID)
			if err != nil {
				return err
			}
			if err := createOrStopIssueStopwatch(e, user, issue); err != nil {
				return err
			}
		}

		// Create stopwatch
		sw = &Stopwatch{
			UserID:  user.ID,
			IssueID: issue.ID,
		}

		if _, err := e.Insert(sw); err != nil {
			return err
		}

		if _, err := createComment(e, &CreateCommentOptions{
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
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := cancelStopwatch(sess, user, issue); err != nil {
		return err
	}
	return sess.Commit()
}

func cancelStopwatch(e *xorm.Session, user *User, issue *Issue) error {
	sw, exists, err := getStopwatch(e, user.ID, issue.ID)
	if err != nil {
		return err
	}

	if exists {
		if _, err := e.Delete(sw); err != nil {
			return err
		}

		if err := issue.loadRepo(e); err != nil {
			return err
		}

		if _, err := createComment(e, &CreateCommentOptions{
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

// SecToTime converts an amount of seconds to a human-readable string (example: 66s -> 1min 6s)
func SecToTime(duration int64) string {
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
