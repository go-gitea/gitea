// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
)

// AutoMerge represents a pull request scheduled for merging when checks succeed
type AutoMerge struct {
	ID          int64                 `xorm:"pk autoincr"`
	PullID      int64                 `xorm:"UNIQUE"`
	DoerID      int64                 `xorm:"NOT NULL"`
	Doer        *user_model.User      `xorm:"-"`
	MergeStyle  repo_model.MergeStyle `xorm:"varchar(30)"`
	Message     string                `xorm:"LONGTEXT"`
	CreatedUnix timeutil.TimeStamp    `xorm:"created"`
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
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pullID int64, style repo_model.MergeStyle, message string) error {
	// Check if we already have a merge scheduled for that pull request
	if exists, _, err := GetScheduledMergeByPullID(ctx, pullID); err != nil {
		return err
	} else if exists {
		return ErrAlreadyScheduledToAutoMerge{PullID: pullID}
	}

	if _, err := db.GetEngine(ctx).Insert(&AutoMerge{
		DoerID:     doer.ID,
		PullID:     pullID,
		MergeStyle: style,
		Message:    message,
	}); err != nil {
		return err
	}

	pr, err := models.GetPullRequestByID(ctx, pullID)
	if err != nil {
		return err
	}

	_, err = createAutoMergeComment(ctx, models.CommentTypePRScheduledToAutoMerge, pr, doer)
	return err
}

// GetScheduledMergeByPullID gets a scheduled pull request merge by pull request id
func GetScheduledMergeByPullID(ctx context.Context, pullID int64) (bool, *AutoMerge, error) {
	scheduledPRM := &AutoMerge{}
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

// RemoveScheduledAutoMerge cancels a previously scheduled pull request
func RemoveScheduledAutoMerge(ctx context.Context, doer *user_model.User, pullID int64, comment bool) error {
	return db.WithTx(func(ctx context.Context) error {
		exist, scheduledPRM, err := GetScheduledMergeByPullID(ctx, pullID)
		if err != nil {
			return err
		} else if !exist {
			return models.ErrNotExist{ID: pullID}
		}

		if _, err := db.GetEngine(ctx).ID(scheduledPRM.ID).Delete(&AutoMerge{}); err != nil {
			return err
		}

		// if pull got merged we don't need to add "auto-merge canceled comment"
		if !comment || doer == nil {
			return nil
		}

		pr, err := models.GetPullRequestByID(ctx, pullID)
		if err != nil {
			return err
		}

		_, err = createAutoMergeComment(ctx, models.CommentTypePRUnScheduledToAutoMerge, pr, doer)
		return err
	}, ctx)
}

// createAutoMergeComment is a internal function, only use it for CommentTypePRScheduledToAutoMerge and CommentTypePRUnScheduledToAutoMerge CommentTypes
func createAutoMergeComment(ctx context.Context, typ models.CommentType, pr *models.PullRequest, doer *user_model.User) (comment *models.Comment, err error) {
	if err = pr.LoadIssueCtx(ctx); err != nil {
		return
	}

	if err = pr.LoadBaseRepoCtx(ctx); err != nil {
		return
	}

	comment, err = models.CreateCommentCtx(ctx, &models.CreateCommentOptions{
		Type:  typ,
		Doer:  doer,
		Repo:  pr.BaseRepo,
		Issue: pr.Issue,
	})
	return
}
