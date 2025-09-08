// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

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

func getStopwatch(ctx context.Context, userID, issueID int64) (sw *Stopwatch, exists bool, err error) {
	sw = new(Stopwatch)
	exists, err = db.GetEngine(ctx).
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(sw)
	return sw, exists, err
}

type UserStopwatch struct {
	UserID      int64
	StopWatches []*Stopwatch
}

func GetUIDsAndStopwatch(ctx context.Context) ([]*UserStopwatch, error) {
	sws := []*Stopwatch{}
	if err := db.GetEngine(ctx).Where("issue_id != 0").Find(&sws); err != nil {
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

// GetUserStopwatches return list of the user's all stopwatches
func GetUserStopwatches(ctx context.Context, userID int64, listOptions db.ListOptions) ([]*Stopwatch, error) {
	sws := make([]*Stopwatch, 0, 8)
	sess := db.GetEngine(ctx).Where("stopwatch.user_id = ?", userID)
	if listOptions.Page > 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	err := sess.Find(&sws)
	if err != nil {
		return nil, err
	}
	return sws, nil
}

// CountUserStopwatches return count of the user's all stopwatches
func CountUserStopwatches(ctx context.Context, userID int64) (int64, error) {
	return db.GetEngine(ctx).Where("user_id = ?", userID).Count(&Stopwatch{})
}

// StopwatchExists returns true if the stopwatch exists
func StopwatchExists(ctx context.Context, userID, issueID int64) bool {
	_, exists, _ := getStopwatch(ctx, userID, issueID)
	return exists
}

// HasUserStopwatch returns true if the user has a stopwatch
func HasUserStopwatch(ctx context.Context, userID int64) (exists bool, sw *Stopwatch, issue *Issue, err error) {
	type stopwatchIssueRepo struct {
		Stopwatch       `xorm:"extends"`
		Issue           `xorm:"extends"`
		repo.Repository `xorm:"extends"`
	}

	swIR := new(stopwatchIssueRepo)
	exists, err = db.GetEngine(ctx).
		Table("stopwatch").
		Where("user_id = ?", userID).
		Join("INNER", "issue", "issue.id = stopwatch.issue_id").
		Join("INNER", "repository", "repository.id = issue.repo_id").
		Get(swIR)
	if exists {
		sw = &swIR.Stopwatch
		issue = &swIR.Issue
		issue.Repo = &swIR.Repository
	}
	return exists, sw, issue, err
}

// FinishIssueStopwatch if stopwatch exists, then finish it.
func FinishIssueStopwatch(ctx context.Context, user *user_model.User, issue *Issue) (ok bool, err error) {
	sw, exists, err := getStopwatch(ctx, user.ID, issue.ID)
	if err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}
	if err = finishIssueStopwatch(ctx, user, issue, sw); err != nil {
		return false, err
	}
	return true, nil
}

func finishIssueStopwatch(ctx context.Context, user *user_model.User, issue *Issue, sw *Stopwatch) error {
	// Create tracked time out of the time difference between start date and actual date
	timediff := time.Now().Unix() - int64(sw.CreatedUnix)

	// Create TrackedTime
	tt := &TrackedTime{
		Created: time.Now(),
		IssueID: issue.ID,
		UserID:  user.ID,
		Time:    timediff,
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}
	if err := db.Insert(ctx, tt); err != nil {
		return err
	}
	if _, err := CreateComment(ctx, &CreateCommentOptions{
		Doer:    user,
		Issue:   issue,
		Repo:    issue.Repo,
		Content: util.SecToHours(timediff),
		Type:    CommentTypeStopTracking,
		TimeID:  tt.ID,
	}); err != nil {
		return err
	}
	_, err := db.DeleteByBean(ctx, sw)
	return err
}

// CreateIssueStopwatch creates a stopwatch if the issue doesn't have the user's stopwatch.
// It also stops any other stopwatch that might be running for the user.
func CreateIssueStopwatch(ctx context.Context, user *user_model.User, issue *Issue) (ok bool, err error) {
	{ // if another issue's stopwatch is running: stop it; if this issue has a stopwatch: return an error.
		exists, otherStopWatch, otherIssue, err := HasUserStopwatch(ctx, user.ID)
		if err != nil {
			return false, err
		}
		if exists {
			if otherStopWatch.IssueID == issue.ID {
				// don't allow starting stopwatch for the same issue
				return false, nil
			}
			// stop the other issue's stopwatch
			if err = finishIssueStopwatch(ctx, user, otherIssue, otherStopWatch); err != nil {
				return false, err
			}
		}
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return false, err
	}
	if err = db.Insert(ctx, &Stopwatch{UserID: user.ID, IssueID: issue.ID}); err != nil {
		return false, err
	}
	if _, err = CreateComment(ctx, &CreateCommentOptions{
		Doer:  user,
		Issue: issue,
		Repo:  issue.Repo,
		Type:  CommentTypeStartTracking,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// CancelStopwatch removes the given stopwatch and logs it into issue's timeline.
func CancelStopwatch(ctx context.Context, user *user_model.User, issue *Issue) (ok bool, err error) {
	err = db.WithTx(ctx, func(ctx context.Context) error {
		e := db.GetEngine(ctx)
		sw, exists, err := getStopwatch(ctx, user.ID, issue.ID)
		if err != nil {
			return err
		} else if !exists {
			return nil
		}

		if err = issue.LoadRepo(ctx); err != nil {
			return err
		}
		if _, err = e.Delete(sw); err != nil {
			return err
		}
		if _, err = CreateComment(ctx, &CreateCommentOptions{
			Doer:  user,
			Issue: issue,
			Repo:  issue.Repo,
			Type:  CommentTypeCancelTracking,
		}); err != nil {
			return err
		}
		ok = true
		return nil
	})
	return ok, err
}
