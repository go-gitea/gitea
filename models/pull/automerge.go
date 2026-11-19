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
	MergedCommitID         string             `xorm:"VARCHAR(64)"`
	ErrorMessage           string             `xorm:"LONGTEXT"` // why the merge attempt failed
}

// TableName return database table name for xorm
func (AutoMerge) TableName() string {
	return "pull_auto_merge"
}

// IsCanceled reports whether the merge attempt failed, the record is only kept to show the error
func (am *AutoMerge) IsCanceled() bool {
	return am.ErrorMessage != ""
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
	if exists, scheduledPRM, err := GetScheduledMergeByPullID(ctx, pullID); err != nil {
		return err
	} else if exists {
		if !scheduledPRM.IsCanceled() {
			return ErrAlreadyScheduledToAutoMerge{PullID: pullID}
		}
		if _, err := DeleteScheduledAutoMerge(ctx, pullID); err != nil {
			return err
		}
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

func GetScheduledMergePullIDsSince(ctx context.Context, since timeutil.TimeStamp) ([]int64, error) {
	var pullIDs []int64
	err := db.GetEngine(ctx).Table(&AutoMerge{}).Where("created_unix >= ?", since).Cols("pull_id").Find(&pullIDs)
	return pullIDs, err
}

func UpdateScheduledAutoMergeError(ctx context.Context, pullID int64, errorMessage string) error {
	_, err := db.GetEngine(ctx).Where("pull_id = ?", pullID).Cols("error_message").Update(&AutoMerge{ErrorMessage: errorMessage})
	return err
}

func DeleteScheduledAutoMerge(ctx context.Context, pullID int64) (int64, error) {
	return db.GetEngine(ctx).Where("pull_id = ?", pullID).Delete(&AutoMerge{})
}
