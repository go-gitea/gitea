// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// NoticeType describes the notice type
type NoticeType int

const (
	// NoticeRepository type
	NoticeRepository NoticeType = iota + 1
	// NoticeTask type
	NoticeTask
)

// Notice represents a system notice for admin.
type Notice struct {
	ID          int64 `xorm:"pk autoincr"`
	Type        NoticeType
	Description string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

func init() {
	db.RegisterModel(new(Notice))
}

// TrStr returns a translation format string.
func (n *Notice) TrStr() string {
	return fmt.Sprintf("admin.notices.type_%d", n.Type)
}

// CreateNotice creates new system notice.
func CreateNotice(ctx context.Context, tp NoticeType, desc string, args ...any) error {
	if len(args) > 0 {
		desc = fmt.Sprintf(desc, args...)
	}
	n := &Notice{
		Type:        tp,
		Description: desc,
	}
	return db.Insert(ctx, n)
}

// CreateRepositoryNotice creates new system notice with type NoticeRepository.
func CreateRepositoryNotice(desc string, args ...any) error {
	// Note we use the db.DefaultContext here rather than passing in a context as the context may be cancelled
	return CreateNotice(db.DefaultContext, NoticeRepository, desc, args...)
}

// RemoveAllWithNotice removes all directories in given path and
// creates a system notice when error occurs.
func RemoveAllWithNotice(ctx context.Context, title, path string) {
	if err := util.RemoveAll(path); err != nil {
		desc := fmt.Sprintf("%s [%s]: %v", title, path, err)
		log.Warn(title+" [%s]: %v", path, err)
		// Note we use the db.DefaultContext here rather than passing in a context as the context may be cancelled
		if err = CreateNotice(db.DefaultContext, NoticeRepository, desc); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
}

// RemoveStorageWithNotice removes a file from the storage and
// creates a system notice when error occurs.
func RemoveStorageWithNotice(ctx context.Context, bucket storage.ObjectStorage, title, path string) {
	if err := bucket.Delete(path); err != nil {
		desc := fmt.Sprintf("%s [%s]: %v", title, path, err)
		log.Warn(title+" [%s]: %v", path, err)

		// Note we use the db.DefaultContext here rather than passing in a context as the context may be cancelled
		if err = CreateNotice(db.DefaultContext, NoticeRepository, desc); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
}

// CountNotices returns number of notices.
func CountNotices() int64 {
	count, _ := db.GetEngine(db.DefaultContext).Count(new(Notice))
	return count
}

// Notices returns notices in given page.
func Notices(page, pageSize int) ([]*Notice, error) {
	notices := make([]*Notice, 0, pageSize)
	return notices, db.GetEngine(db.DefaultContext).
		Limit(pageSize, (page-1)*pageSize).
		Desc("created_unix").
		Find(&notices)
}

// DeleteNotice deletes a system notice by given ID.
func DeleteNotice(id int64) error {
	_, err := db.GetEngine(db.DefaultContext).ID(id).Delete(new(Notice))
	return err
}

// DeleteNotices deletes all notices with ID from start to end (inclusive).
func DeleteNotices(start, end int64) error {
	if start == 0 && end == 0 {
		_, err := db.GetEngine(db.DefaultContext).Exec("DELETE FROM notice")
		return err
	}

	sess := db.GetEngine(db.DefaultContext).Where("id >= ?", start)
	if end > 0 {
		sess.And("id <= ?", end)
	}
	_, err := sess.Delete(new(Notice))
	return err
}

// DeleteNoticesByIDs deletes notices by given IDs.
func DeleteNoticesByIDs(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := db.GetEngine(db.DefaultContext).
		In("id", ids).
		Delete(new(Notice))
	return err
}

// DeleteOldSystemNotices deletes all old system notices from database.
func DeleteOldSystemNotices(olderThan time.Duration) (err error) {
	if olderThan <= 0 {
		return nil
	}

	_, err = db.GetEngine(db.DefaultContext).Where("created_unix < ?", time.Now().Add(-olderThan).Unix()).Delete(&Notice{})
	return err
}
