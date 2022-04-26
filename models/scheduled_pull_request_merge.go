// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
)

// ScheduledPullRequestMerge represents a pull request scheduled for merging when checks succeed
type ScheduledPullRequestMerge struct {
	ID          int64                 `xorm:"pk autoincr"`
	PullID      int64                 `xorm:"BIGINT"`
	DoerID      int64                 `xorm:"BIGINT"`
	Doer        *user_model.User      `xorm:"-"`
	MergeStyle  repo_model.MergeStyle `xorm:"varchar(50)"`
	Message     string                `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp    `xorm:"created"`
}

func init() {
	db.RegisterModel(new(ScheduledPullRequestMerge))
}

// ScheduleAutoMerge schedules a pull request to be merged when all checks succeed
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pullID int64, style repo_model.MergeStyle, message string) error {
	// Check if we already have a merge scheduled for that pull request
	if exists, _, err := GetScheduledPullRequestMergeByPullID(ctx, pullID); err != nil {
		return err
	} else if exists {
		return ErrPullRequestAlreadyScheduledToAutoMerge{PullID: pullID}
	}

	if _, err := db.GetEngine(ctx).Insert(&ScheduledPullRequestMerge{
		DoerID:     doer.ID,
		PullID:     pullID,
		MergeStyle: style,
		Message:    message,
	}); err != nil {
		return err
	}

	pr, err := getPullRequestByID(db.GetEngine(ctx), pullID)
	if err != nil {
		return err
	}

	_, err = createAutoMergeComment(ctx, CommentTypePRScheduledToAutoMerge, pr, doer)
	return err
}

// GetScheduledPullRequestMergeByPullID gets a scheduled pull request merge by pull request id
func GetScheduledPullRequestMergeByPullID(ctx context.Context, pullID int64) (bool, *ScheduledPullRequestMerge, error) {
	scheduledPRM := &ScheduledPullRequestMerge{}
	exists, err := db.GetEngine(ctx).Where("pull_id = ?", pullID).Get(scheduledPRM)
	if err != nil || !exists {
		return false, nil, err
	}

	doer, err := user_model.GetUserByIDCtx(ctx, scheduledPRM.DoerID)
	if err != nil {
		return false, nil, err
	}

	scheduledPRM.Doer = doer
	return true, scheduledPRM, nil
}

// RemoveScheduledPullRequestMerge cancels a previously scheduled pull request
func RemoveScheduledPullRequestMerge(ctx context.Context, doer *user_model.User, pullID int64, comment bool) error {
	exist, scheduledPRM, err := GetScheduledPullRequestMergeByPullID(ctx, pullID)
	if err != nil {
		return err
	} else if !exist {
		return ErrNotExist{ID: pullID}
	}

	if _, err := db.GetEngine(ctx).ID(scheduledPRM.ID).Delete(&ScheduledPullRequestMerge{}); err != nil {
		return err
	}

	// if pull got merged we dont need to add a "auto-merge canceled comment"
	if !comment || doer == nil {
		return nil
	}

	pr, err := getPullRequestByID(db.GetEngine(ctx), pullID)
	if err != nil {
		return err
	}

	_, err = createAutoMergeComment(ctx, CommentTypePRUnScheduledToAutoMerge, pr, doer)
	return err
}
