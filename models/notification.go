package models

import (
	"time"
)

const (
	NotificationStatusUnread = "U"
	NotificationStatusRead   = "R"

	NotificationSourceIssue       = "I"
	NotificationSourcePullRequest = "P"
	NotificationSourceCommit      = "C"
)

type Notification struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX NOT NULL"`
	RepoID int64 `xorm:"INDEX NOT NULL"`

	Status string `xorm:"VARCHAR(1) INDEX NOT NULL"`
	Source string `xorm:"VARCHAR(1) INDEX NOT NULL"`

	IssueID  int64  `xorm:"INDEX NOT NULL"`
	PullID   int64  `xorm:"INDEX"`
	CommitID string `xorm:"INDEX"`

	Issue       *Issue       `xorm:"-"`
	PullRequest *PullRequest `xorm:"-"`

	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"INDEX NOT NULL"`
	Updated     time.Time `xorm:"-"`
	UpdatedUnix int64     `xorm:"INDEX NOT NULL"`
}

func (n *Notification) BeforeInsert() {
	var (
		now     = time.Now()
		nowUnix = now.Unix()
	)
	n.Created = now
	n.CreatedUnix = nowUnix
	n.Updated = now
	n.UpdatedUnix = nowUnix
}
func (n *Notification) BeforeUpdate() {
	var (
		now     = time.Now()
		nowUnix = now.Unix()
	)
	n.Updated = now
	n.UpdatedUnix = nowUnix
}
