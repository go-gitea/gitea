package models

import (
	"time"
)

// IssueWatch is connection request for receiving issue notification.
type IssueWatch struct {
	ID          int64     `xorm:"pk autoincr"`
	UserID      int64     `xorm:"UNIQUE(watch) NOT NULL"`
	IssueID     int64     `xorm:"UNIQUE(watch) NOT NULL"`
	IsWatching  bool      `xorm:"NOT NULL"`
	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"NOT NULL"`
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (iw *IssueWatch) BeforeInsert() {
	iw.CreatedUnix = time.Now().Unix()
}
