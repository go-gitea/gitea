// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

type PushMirrorStatus string

const (
	PushMirrorStatusSuccess PushMirrorStatus = "success"
	PushMirrorStatusPartial PushMirrorStatus = "partial"
	PushMirrorStatusFailed  PushMirrorStatus = "failed"
)

// PushMirrorRefError is a ref which failed to be pushed
type PushMirrorRefError struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// PushMirrorResult is the detailed outcome of a push mirror sync
type PushMirrorResult struct {
	Pushed      int                  `json:"pushed"`
	Deleted     int                  `json:"deleted"`
	UpToDate    int                  `json:"up_to_date"`
	FailedTotal int                  `json:"failed_total"`
	Failed      []PushMirrorRefError `json:"failed,omitempty"` // capped, see FailedTotal
	Error       string               `json:"error,omitempty"`  // the whole sync failed
}

// PushMirrorHistory records one run of a push mirror
type PushMirrorHistory struct {
	ID           int64 `xorm:"pk autoincr"`
	RepoID       int64 `xorm:"INDEX"`
	PushMirrorID int64 `xorm:"INDEX"`
	Status       PushMirrorStatus
	DurationMs   int64
	Result       PushMirrorResult   `xorm:"TEXT JSON"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(PushMirrorHistory))
}

// AddPushMirrorHistory stores a history record and drops the oldest ones beyond keep
func AddPushMirrorHistory(ctx context.Context, h *PushMirrorHistory, keep int) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := db.Insert(ctx, h); err != nil {
			return err
		}
		var oldest []int64
		if err := db.GetEngine(ctx).Table("push_mirror_history").Cols("id").
			Where("push_mirror_id = ?", h.PushMirrorID).
			Desc("id").Limit(1, max(keep, 1)).Find(&oldest); err != nil || len(oldest) == 0 {
			return err
		}
		_, err := db.GetEngine(ctx).Where("push_mirror_id = ? AND id <= ?", h.PushMirrorID, oldest[0]).Delete(&PushMirrorHistory{})
		return err
	})
}

// GetPushMirrorHistory returns the history of a push mirror, newest first
func GetPushMirrorHistory(ctx context.Context, pushMirrorID int64) ([]*PushMirrorHistory, error) {
	var list []*PushMirrorHistory
	return list, db.GetEngine(ctx).Where("push_mirror_id = ?", pushMirrorID).Desc("id").Find(&list)
}
