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
	Updated     time.Time `xorm:"-"`
	UpdatedUnix int64     `xorm:"NOT NULL"`
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (iw *IssueWatch) BeforeInsert() {
	iw.Created = time.Now()
	iw.CreatedUnix = time.Now().Unix()
	iw.Updated = time.Now()
	iw.UpdatedUnix = time.Now().Unix()
}

func (iw *IssueWatch) BeforeUpdate() {
	iw.Updated = time.Now()
	iw.UpdatedUnix = time.Now().Unix()
}

// CreateOrUpdateIssueWatch set watching for a user and issue
func CreateOrUpdateIssueWatch(userID, issueID int64, isWatching bool) error {
	iw, exists, err := getIssueWatch(x, userID, issueID)
	if err != nil {
		return err
	}

	if !exists {
		iw = &IssueWatch{
			UserID:     userID,
			IssueID:    issueID,
			IsWatching: isWatching,
		}

		if _, err := x.Insert(iw); err != nil {
			return err
		}
	} else {
		iw.IsWatching = isWatching

		if _, err := x.Id(iw.ID).Cols("is_watching", "updated_unix").Update(iw); err != nil {
			return err
		}
	}
	return nil
}

// GetIssueWatch returns an issue watch by user and issue
func GetIssueWatch(userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	iw, exists, err = getIssueWatch(x, userID, issueID)
	return
}
func getIssueWatch(e Engine, userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	iw = new(IssueWatch)
	exists, err = e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(iw)
	return
}

func GetIssueWatchers(issueID int64) ([]*IssueWatch, error) {
	return getIssueWatchers(x, issueID)
}
func getIssueWatchers(e Engine, issueID int64) (watches []*IssueWatch, err error) {
	err = e.
		Where("issue_id = ?", issueID).
		Find(&watches)
	return
}
