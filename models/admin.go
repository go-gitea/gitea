// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

//NoticeType describes the notice type
type NoticeType int

const (
	//NoticeRepository type
	NoticeRepository NoticeType = iota + 1
)

// Notice represents a system notice for admin.
type Notice struct {
	ID          int64 `xorm:"pk autoincr"`
	Type        NoticeType
	Description string    `xorm:"TEXT"`
	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"INDEX"`
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (n *Notice) BeforeInsert() {
	n.CreatedUnix = time.Now().Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (n *Notice) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		n.Created = time.Unix(n.CreatedUnix, 0).Local()
	}
}

// TrStr returns a translation format string.
func (n *Notice) TrStr() string {
	return "admin.notices.type_" + com.ToStr(n.Type)
}

// CreateNotice creates new system notice.
func CreateNotice(tp NoticeType, desc string) error {
	// prevent panic if database connection is not available at this point
	if x == nil {
		return fmt.Errorf("Could not save notice due database connection not being available: %d %s", tp, desc)
	}

	n := &Notice{
		Type:        tp,
		Description: desc,
	}
	_, err := x.Insert(n)
	return err
}

// CreateRepositoryNotice creates new system notice with type NoticeRepository.
func CreateRepositoryNotice(desc string) error {
	return CreateNotice(NoticeRepository, desc)
}

// RemoveAllWithNotice removes all directories in given path and
// creates a system notice when error occurs.
func RemoveAllWithNotice(title, path string) {
	var err error
	// workaround for Go not being able to remove read-only files/folders: https://github.com/golang/go/issues/9606
	// this bug should be fixed on Go 1.7, so the workaround should be removed when Gogs don't support Go 1.6 anymore:
	// https://github.com/golang/go/commit/2ffb3e5d905b5622204d199128dec06cefd57790
	if setting.IsWindows {
		// converting "/" to "\" in path on Windows
		path = strings.Replace(path, "/", "\\", -1)
		err = exec.Command("cmd", "/C", "rmdir", "/S", "/Q", path).Run()
	} else {
		err = os.RemoveAll(path)
	}

	if err != nil {
		desc := fmt.Sprintf("%s [%s]: %v", title, path, err)
		log.Warn(desc)
		if err = CreateRepositoryNotice(desc); err != nil {
			log.Error(4, "CreateRepositoryNotice: %v", err)
		}
	}
}

// CountNotices returns number of notices.
func CountNotices() int64 {
	count, _ := x.Count(new(Notice))
	return count
}

// Notices returns notices in given page.
func Notices(page, pageSize int) ([]*Notice, error) {
	notices := make([]*Notice, 0, pageSize)
	return notices, x.
		Limit(pageSize, (page-1)*pageSize).
		Desc("id").
		Find(&notices)
}

// DeleteNotice deletes a system notice by given ID.
func DeleteNotice(id int64) error {
	_, err := x.Id(id).Delete(new(Notice))
	return err
}

// DeleteNotices deletes all notices with ID from start to end (inclusive).
func DeleteNotices(start, end int64) error {
	sess := x.Where("id >= ?", start)
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
	_, err := x.
		In("id", ids).
		Delete(new(Notice))
	return err
}
