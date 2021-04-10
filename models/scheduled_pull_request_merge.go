// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"
)

// ScheduledPullRequestMerge represents a pull request scheduled for merging when checks succeed
type ScheduledPullRequestMerge struct {
	ID          int64              `xorm:"pk autoincr"`
	PullID      int64              `xorm:"BIGINT"`
	DoerID      int64              `xorm:"BIGINT"`
	Doer        *User              `xorm:"-"`
	MergeStyle  MergeStyle         `xorm:"varchar(50)"`
	Message     string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// ScheduleAutoMerge schedules a pull request to be merged when all checks succeed
func ScheduleAutoMerge(opts *ScheduledPullRequestMerge) (err error) {
	if opts.Doer == nil {
		return fmt.Errorf("ScheduleAutoMerge need Doer")
	}
	opts.DoerID = opts.Doer.ID

	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer sess.Close()

	// Check if we already have a merge scheduled for that pull request
	exists, err := sess.Exist(&ScheduledPullRequestMerge{PullID: opts.PullID})
	if err != nil {
		return
	}
	if exists {
		return ErrPullRequestAlreadyScheduledToAutoMerge{PullID: opts.PullID}
	}

	if _, err = sess.Insert(opts); err != nil {
		return
	}

	pr, err := getPullRequestByID(sess, opts.PullID)
	if err != nil {
		return err
	}

	if _, err := createAutoMergeComment(sess, CommentTypePRScheduledToAutoMerge, pr, opts.Doer); err != nil {
		return err
	}

	return sess.Commit()
}

// GetScheduledMergeRequestByPullID gets a scheduled pull request merge by pull request id
func GetScheduledMergeRequestByPullID(pullID int64) (bool, *ScheduledPullRequestMerge, error) {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return false, nil, err
	}
	defer sess.Close()

	scheduledPRM := &ScheduledPullRequestMerge{}
	exists, err := sess.Where("pull_id = ?", pullID).Get(scheduledPRM)
	if err != nil || !exists {
		return false, nil, err
	}

	if doer, err := getUserByID(sess, scheduledPRM.DoerID); err != nil {
		return false, nil, err
	} else {
		scheduledPRM.Doer = doer
	}

	return true, scheduledPRM, nil
}

// RemoveScheduledMergeRequest cancels a previously scheduled pull request
func RemoveScheduledMergeRequest(scheduledPR *ScheduledPullRequestMerge) error {
	if scheduledPR.ID != 0 {
		_, err := x.ID(scheduledPR.ID).Delete(&ScheduledPullRequestMerge{})
		return err
	}

	_, err := x.Where("pull_id = ?", scheduledPR.PullID).Delete(&ScheduledPullRequestMerge{})
	return err
}
