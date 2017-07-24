package models

import (
	"code.gitea.io/gitea/modules/log"
	"github.com/go-xorm/xorm"
	"time"
)

// TrackedTime represents a time that was spent for a specific issue.
type TrackedTime struct {
	ID          int64     `xorm:"pk autoincr"`
	IssueID     int64     `xorm:"INDEX"`
	Issue       *Issue    `xorm:"-"`
	UserID      int64     `xorm:"INDEX"`
	User        *User     `xorm:"-"`
	Created     time.Time `xorm:"-"`
	CreatedUnix int64
	Time        int64
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (t *TrackedTime) AfterSet(colName string, _ xorm.Cell) {
	var err error
	switch colName {
	case "user_id":
		t.User, err = GetUserByID(t.UserID)
		if err != nil {
			if IsErrUserNotExist(err) {
				t.UserID = -1
				t.User = nil
			} else {
				log.Error(3, "GetUserByID[%d]: %v", t.UserID, err)
			}
		}

	case "issue_id":
		t.Issue, err = GetIssueByID(t.IssueID)
		if err != nil {
			if IsErrIssueNotExist(err) {
				t.IssueID = -1
				t.Issue = nil
			} else {
				log.Error(3, "GetIssueByID[%d]: %v", t.IssueID, err)
			}
		}
	case "created_unix":
		t.Created = time.Unix(t.CreatedUnix, 0).Local()
	}
}

// GetTrackedTimeByID returns the tracked time by given ID.
func GetTrackedTimeByID(id int64) (*TrackedTime, error) {
	c := new(TrackedTime)
	has, err := x.Id(id).Get(c)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTrackedTimeNotExist{id}
	}
	return c, nil
}

// BeforeInsert will be invoked by XORM before inserting a record
// representing this object.
func (t *TrackedTime) BeforeInsert() {
	t.CreatedUnix = time.Now().Unix()
}
