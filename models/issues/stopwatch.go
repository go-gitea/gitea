// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// ErrIssueStopwatchNotExist represents an error that stopwatch is not exist
type ErrIssueStopwatchNotExist struct {
	UserID  int64
	IssueID int64
}

func (err ErrIssueStopwatchNotExist) Error() string {
	return fmt.Sprintf("issue stopwatch doesn't exist[uid: %d, issue_id: %d", err.UserID, err.IssueID)
}

func (err ErrIssueStopwatchNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrIssueStopwatchAlreadyExist represents an error that stopwatch is already exist
type ErrIssueStopwatchAlreadyExist struct {
	UserID  int64
	IssueID int64
}

func (err ErrIssueStopwatchAlreadyExist) Error() string {
	return fmt.Sprintf("issue stopwatch already exists[uid: %d, issue_id: %d", err.UserID, err.IssueID)
}

func (err ErrIssueStopwatchAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// Stopwatch represents a stopwatch for time tracking.
type Stopwatch struct {
	ID          int64              `xorm:"pk autoincr"`
	IssueID     int64              `xorm:"INDEX"`
	UserID      int64              `xorm:"INDEX"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(Stopwatch))
}

// Seconds returns the amount of time passed since creation, based on local server time
func (s Stopwatch) Seconds() int64 {
	return int64(timeutil.TimeStampNow() - s.CreatedUnix)
}

// Duration returns a human-readable duration string based on local server time
func (s Stopwatch) Duration() string {
	return util.SecToTime(s.Seconds())
}

func getStopwatch(ctx context.Context, userID, issueID int64) (sw *Stopwatch, exists bool, err error) {
	sw = new(Stopwatch)
	exists, err = db.GetEngine(ctx).
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(sw)
	return sw, exists, err
}

// UserIDCount is a simple coalition of UserID and Count
type UserStopwatch struct {
	UserID      int64
	StopWatches []*Stopwatch
}

// GetUIDsAndNotificationCounts between the two provided times
func GetUIDsAndStopwatch() ([]*UserStopwatch, error) {
	sws := []*Stopwatch{}
	if err := db.GetEngine(db.DefaultContext).Where("issue_id != 0").Find(&sws); err != nil {
		return nil, err
	}
	if len(sws) == 0 {
		return []*UserStopwatch{}, nil
	}

	lastUserID := int64(-1)
	res := []*UserStopwatch{}
	for _, sw := range sws {
		if lastUserID == sw.UserID {
			lastUserStopwatch := res[len(res)-1]
			lastUserStopwatch.StopWatches = append(lastUserStopwatch.StopWatches, sw)
		} else {
			res = append(res, &UserStopwatch{
				UserID:      sw.UserID,
				StopWatches: []*Stopwatch{sw},
			})
		}
	}
	return res, nil
}

// GetUserStopwatches return list of all stopwatches of a user
func GetUserStopwatches(userID int64, listOptions db.ListOptions) ([]*Stopwatch, error) {
	sws := make([]*Stopwatch, 0, 8)
	sess := db.GetEngine(db.DefaultContext).Where("stopwatch.user_id = ?", userID)
	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	err := sess.Find(&sws)
	if err != nil {
		return nil, err
	}
	return sws, nil
}

// CountUserStopwatches return count of all stopwatches of a user
func CountUserStopwatches(userID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("user_id = ?", userID).Count(&Stopwatch{})
}

// StopwatchExists returns true if the stopwatch exists
func StopwatchExists(userID, issueID int64) bool {
	_, exists, _ := getStopwatch(db.DefaultContext, userID, issueID)
	return exists
}

// HasUserStopwatch returns true if the user has a stopwatch
func HasUserStopwatch(ctx context.Context, userID int64) (exists bool, sw *Stopwatch, err error) {
	sw = new(Stopwatch)
	exists, err = db.GetEngine(ctx).
		Where("user_id = ?", userID).
		Get(sw)
	return exists, sw, err
}

// FinishIssueStopwatchIfPossible if stopwatch exist then finish it otherwise ignore
func FinishIssueStopwatchIfPossible(ctx context.Context, user *user_model.User, issue *Issue) error {
	_, exists, err := getStopwatch(ctx, user.ID, issue.ID)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return FinishIssueStopwatch(ctx, user, issue)
}

// CreateOrStopIssueStopwatch create an issue stopwatch if it's not exist, otherwise finish it
func CreateOrStopIssueStopwatch(user *user_model.User, issue *Issue) error {
	_, exists, err := getStopwatch(db.DefaultContext, user.ID, issue.ID)
	if err != nil {
		return err
	}
	if exists {
		return FinishIssueStopwatch(db.DefaultContext, user, issue)
	}
	return CreateIssueStopwatch(db.DefaultContext, user, issue)
}

// FinishIssueStopwatch if stopwatch exist then finish it otherwise return an error
func FinishIssueStopwatch(ctx context.Context, user *user_model.User, issue *Issue) error {
	sw, exists, err := getStopwatch(ctx, user.ID, issue.ID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrIssueStopwatchNotExist{
			UserID:  user.ID,
			IssueID: issue.ID,
		}
	}

	// Create tracked time out of the time difference between start date and actual date
	timediff := time.Now().Unix() - int64(sw.CreatedUnix)

	// Create TrackedTime
	tt := &TrackedTime{
		Created: time.Now(),
		IssueID: issue.ID,
		UserID:  user.ID,
		Time:    timediff,
	}

	if err := db.Insert(ctx, tt); err != nil {
		return err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	if _, err := CreateComment(ctx, &CreateCommentOptions{
		Doer:        user,
		Issue:       issue,
		Repo:        issue.Repo,
		Type:        CommentTypeStopTracking,
		TimeID:      tt.ID,
		TimeTracked: tt.Time,
	}); err != nil {
		return err
	}
	_, err = db.DeleteByBean(ctx, sw)
	return err
}

// CreateIssueStopwatch creates a stopwatch if not exist, otherwise return an error
func CreateIssueStopwatch(ctx context.Context, user *user_model.User, issue *Issue) error {
	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	// if another stopwatch is running: stop it
	exists, sw, err := HasUserStopwatch(ctx, user.ID)
	if err != nil {
		return err
	}
	if exists {
		issue, err := GetIssueByID(ctx, sw.IssueID)
		if err != nil {
			return err
		}

		if err := FinishIssueStopwatch(ctx, user, issue); err != nil {
			return err
		}
	}

	// Create stopwatch
	sw = &Stopwatch{
		UserID:  user.ID,
		IssueID: issue.ID,
	}

	if err := db.Insert(ctx, sw); err != nil {
		return err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	if _, err := CreateComment(ctx, &CreateCommentOptions{
		Doer:  user,
		Issue: issue,
		Repo:  issue.Repo,
		Type:  CommentTypeStartTracking,
	}); err != nil {
		return err
	}

	return nil
}

// CancelStopwatch removes the given stopwatch and logs it into issue's timeline.
func CancelStopwatch(user *user_model.User, issue *Issue) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	if err := cancelStopwatch(ctx, user, issue); err != nil {
		return err
	}
	return committer.Commit()
}

func cancelStopwatch(ctx context.Context, user *user_model.User, issue *Issue) error {
	e := db.GetEngine(ctx)
	sw, exists, err := getStopwatch(ctx, user.ID, issue.ID)
	if err != nil {
		return err
	}

	if exists {
		if _, err := e.Delete(sw); err != nil {
			return err
		}

		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		if _, err := CreateComment(ctx, &CreateCommentOptions{
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
