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

// AddTime will add the given time (in seconds) to the issue
func AddTime(userID int64, issueID int64, time int64) error {
	tt := &TrackedTime{
		IssueID: issueID,
		UserID:  userID,
		Time:    time,
	}
	if _, err := x.Insert(tt); err != nil {
		return err
	}
	comment := &Comment{
		IssueID:  issueID,
		PosterID: userID,
		Type:     CommentTypeAddTimeManual,
		Content:  secToTime(time),
	}
	if _, err := x.Insert(comment); err != nil {
		return err
	}
	return nil
}

// TotalTimes returns the spent time for each user by an issue
func TotalTimes(issueID int64) (map[*User]string, error) {
	var trackedTimes []TrackedTime
	if err := x.
		Where("issue_id = ?", issueID).
		Find(&trackedTimes); err != nil {
		return nil, err
	}
	//Adding total time per user ID
	totalTimesByUser := make(map[int64]int64)
	for _, t := range trackedTimes {
		if total, ok := totalTimesByUser[t.UserID]; !ok {
			totalTimesByUser[t.UserID] = t.Time
		} else {
			totalTimesByUser[t.UserID] = total + t.Time
		}
	}

	totalTimes := make(map[*User]string)
	//Fetching User and making time human readable
	for userID, total := range totalTimesByUser {
		user, err := GetUserByID(userID)
		if err != nil || user == nil {
			continue
		}
		totalTimes[user] = secToTime(total)
	}
	return totalTimes, nil
}
