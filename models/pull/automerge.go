// Copyright 2022 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"
)

// AutoMerge represents a pull request scheduled for merging when checks succeed
type AutoMerge struct {
	ID                     int64                 `xorm:"pk autoincr"`
	PullID                 int64                 `xorm:"UNIQUE"`
	DoerID                 int64                 `xorm:"INDEX NOT NULL"`
	Doer                   *user_model.User      `xorm:"-"`
	MergeStyle             repo_model.MergeStyle `xorm:"varchar(30)"`
	Message                string                `xorm:"LONGTEXT"`
	DeleteBranchAfterMerge bool
	CreatedUnix            timeutil.TimeStamp `xorm:"created"`
}

// TableName return database table name for xorm
func (AutoMerge) TableName() string {
	return "pull_auto_merge"
}

func init() {
	db.RegisterModel(new(AutoMerge))
}

// ErrAlreadyScheduledToAutoMerge represents a "PullRequestHasMerged"-error
type ErrAlreadyScheduledToAutoMerge struct {
	PullID int64
}

func (err ErrAlreadyScheduledToAutoMerge) Error() string {
	return fmt.Sprintf("pull request is already scheduled to auto merge when checks succeed [pull_id: %d]", err.PullID)
}

// IsErrAlreadyScheduledToAutoMerge checks if an error is a ErrAlreadyScheduledToAutoMerge.
func IsErrAlreadyScheduledToAutoMerge(err error) bool {
	_, ok := err.(ErrAlreadyScheduledToAutoMerge)
	return ok
}

// ScheduleAutoMerge schedules a pull request to be merged when all checks succeed
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pullID int64, style repo_model.MergeStyle, message string, deleteBranchAfterMerge bool) error {
	// Check if we already have a merge scheduled for that pull request
	if exists, _, err := GetScheduledMergeByPullID(ctx, pullID); err != nil {
		return err
	} else if exists {
		return ErrAlreadyScheduledToAutoMerge{PullID: pullID}
	}

	_, err := db.GetEngine(ctx).Insert(&AutoMerge{
		DoerID:                 doer.ID,
		PullID:                 pullID,
		MergeStyle:             style,
		Message:                message,
		DeleteBranchAfterMerge: deleteBranchAfterMerge,
	})
	return err
}

// GetScheduledMergeByPullID gets a scheduled pull request merge by pull request id
func GetScheduledMergeByPullID(ctx context.Context, pullID int64) (bool, *AutoMerge, error) {
	scheduledPRM := &AutoMerge{}
	exists, err := db.GetEngine(ctx).Where("pull_id = ?", pullID).Get(scheduledPRM)
	if err != nil || !exists {
		return false, nil, err
	}

	scheduledPRM.DoerID, scheduledPRM.Doer, err = user_model.GetPossibleUserByID(ctx, scheduledPRM.DoerID)
	return true, scheduledPRM, err
}

// GetScheduledMergeByPullIDs returns the scheduled auto merges for the given pull request IDs, keyed by pull ID.
// The returned entries do not have their Doer loaded.
func GetScheduledMergeByPullIDs(ctx context.Context, pullIDs []int64) (map[int64]*AutoMerge, error) {
	if len(pullIDs) == 0 {
		return map[int64]*AutoMerge{}, nil
	}
	merges := make([]*AutoMerge, 0, len(pullIDs))
	if err := db.GetEngine(ctx).In("pull_id", pullIDs).Find(&merges); err != nil {
		return nil, err
	}
	result := make(map[int64]*AutoMerge, len(merges))
	for _, m := range merges {
		result[m.PullID] = m
	}
	return result, nil
}

// DeleteScheduledAutoMerge delete a scheduled pull request
func DeleteScheduledAutoMerge(ctx context.Context, pullID int64) error {
	exist, scheduledPRM, err := GetScheduledMergeByPullID(ctx, pullID)
	if err != nil {
		return err
	} else if !exist {
		return db.ErrNotExist{Resource: "auto_merge", ID: pullID}
	}

	_, err = db.GetEngine(ctx).ID(scheduledPRM.ID).Delete(&AutoMerge{})
	return err
}
