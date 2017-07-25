package models

import (
	"github.com/go-xorm/xorm"
	"time"
)

// TrackedTime represents a time that was spent for a specific issue.
type TrackedTime struct {
	ID          int64     `xorm:"pk autoincr"`
	IssueID     int64     `xorm:"INDEX"`
	UserID      int64     `xorm:"INDEX"`
	Created     time.Time `xorm:"-"`
	CreatedUnix int64
	Time        int64
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (t *TrackedTime) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
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
