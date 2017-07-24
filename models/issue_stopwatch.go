package models

import (
	"time"
	"github.com/go-xorm/xorm"
	"code.gitea.io/gitea/modules/log"
)

// Stopwatch represents a stopwatch for time tracking.
type Stopwatch struct {
	ID              int64 `xorm:"pk autoincr"`
	IssueID         int64 `xorm:"INDEX"`
	Issue		*Issue `xorm:"-"`
	UserID          int64 `xorm:"INDEX"`
	User		*User `xorm:"-"`
	Created       	time.Time `xorm:"-"`
	CreatedUnix   	int64
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (s *Stopwatch) AfterSet(colName string, _ xorm.Cell) {
	var err error
	switch colName {
	case "user_id":
		s.User, err = GetUserByID(s.UserID)
		if err != nil {
			if IsErrUserNotExist(err) {
				s.UserID = -1
				s.User = nil
			} else {
				log.Error(3, "GetUserByID[%d]: %v", s.UserID, err)
			}
		}

	case "issue_id":
		s.Issue, err = GetIssueByID(s.IssueID)
		if err != nil {
			if IsErrIssueNotExist(err) {
				s.IssueID = -1
				s.Issue = nil
			} else {
				log.Error(3, "GetIssueByID[%d]: %v", s.IssueID, err)
			}
		}
	case "created_unix":
		s.Created = time.Unix(s.CreatedUnix, 0).Local()
	}
}

// GetStopwatchByID returns the stopwatch by given ID.
func GetStopwatchByID(id int64) (*Stopwatch, error) {
	c := new(Stopwatch)
	has, err := x.Id(id).Get(c)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrStopwatchNotExist{id}
	}
	return c, nil
}

// BeforeInsert will be invoked by XORM before inserting a record
// representing this object.
func (s *Stopwatch) BeforeInsert() {
	s.CreatedUnix = time.Now().Unix()
}
