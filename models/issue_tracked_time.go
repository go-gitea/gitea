// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// TrackedTime represents a time that was spent for a specific issue.
type TrackedTime struct {
	ID          int64     `xorm:"pk autoincr" json:"id"`
	IssueID     int64     `xorm:"INDEX" json:"issue_id"`
	UserID      int64     `xorm:"INDEX" json:"user_id"`
	Created     time.Time `xorm:"-" json:"created"`
	CreatedUnix int64     `xorm:"created" json:"-"`
	Time        int64     `json:"time"`
}

// TrackedTimeList is a List ful of TrackedTime's
type TrackedTimeList []*TrackedTime

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (t *TrackedTime) AfterLoad() {
	t.Created = time.Unix(t.CreatedUnix, 0).In(setting.DefaultUILocation)
}

// APIFormat converts TrackedTime to API format
func (t *TrackedTime) APIFormat() *api.TrackedTime {
	user, err := GetUserByID(t.UserID)
	if err != nil {
		return nil
	}
	issue, err := GetIssueByID(t.IssueID)
	if err != nil {
		return nil
	}
	err = issue.LoadRepo()
	if err != nil {
		return nil
	}
	return &api.TrackedTime{
		ID:       t.ID,
		IssueID:  t.IssueID,
		Issue:    issue.APIFormat(),
		UserID:   t.UserID,
		UserName: user.Name,
		Time:     t.Time,
		Created:  t.Created,
	}
}

// APIFormat converts TrackedTimeList to API format
func (tl TrackedTimeList) APIFormat() api.TrackedTimeList {
	var result api.TrackedTimeList
	for _, t := range tl {
		i := t.APIFormat()
		if i != nil {
			result = append(result, i)
		}
	}
	return result
}

// FindTrackedTimesOptions represent the filters for tracked times. If an ID is 0 it will be ignored.
type FindTrackedTimesOptions struct {
	IssueID      int64
	UserID       int64
	RepositoryID int64
	MilestoneID  int64
}

// ToCond will convert each condition into a xorm-Cond
func (opts *FindTrackedTimesOptions) ToCond() builder.Cond {
	cond := builder.NewCond()
	if opts.IssueID != 0 {
		cond = cond.And(builder.Eq{"issue_id": opts.IssueID})
	}
	if opts.UserID != 0 {
		cond = cond.And(builder.Eq{"user_id": opts.UserID})
	}
	if opts.RepositoryID != 0 {
		cond = cond.And(builder.Eq{"issue.repo_id": opts.RepositoryID})
	}
	if opts.MilestoneID != 0 {
		cond = cond.And(builder.Eq{"issue.milestone_id": opts.MilestoneID})
	}
	return cond
}

// ToSession will convert the given options to a xorm Session by using the conditions from ToCond and joining with issue table if required
func (opts *FindTrackedTimesOptions) ToSession(e Engine) *xorm.Session {
	if opts.RepositoryID > 0 || opts.MilestoneID > 0 {
		return e.Join("INNER", "issue", "issue.id = tracked_time.issue_id").Where(opts.ToCond())
	}
	return x.Where(opts.ToCond())
}

// GetTrackedTimes returns all tracked times that fit to the given options.
func GetTrackedTimes(options FindTrackedTimesOptions) (trackedTimes TrackedTimeList, err error) {
	err = options.ToSession(x).Find(&trackedTimes)
	return
}

// GetTrackedSeconds return sum of seconds
func GetTrackedSeconds(options FindTrackedTimesOptions) (trackedSeconds int64, err error) {
	var trackedTimes TrackedTimeList
	err = options.ToSession(x).Find(&trackedTimes)
	if err != nil {
		return 0, err
	}
	for _, t := range trackedTimes {
		trackedSeconds += t.Time
	}
	return trackedSeconds, nil
}

// AddTime will add the given time (in seconds) to the issue
func AddTime(user *User, issue *Issue, amount int64) (*TrackedTime, error) {
	return addTime(user, issue, amount, time.Now())
}

// AddTimeCreatedAt will add the given time (in seconds) to the issue at a specific time
func AddTimeCreatedAt(user *User, issue *Issue, amount int64, created time.Time) (*TrackedTime, error) {
	return addTime(user, issue, amount, created)
}

func addTime(user *User, issue *Issue, amount int64, created time.Time) (*TrackedTime, error) {
	if created.IsZero() {
		created = time.Now()
	}
	tt := &TrackedTime{
		IssueID: issue.ID,
		UserID:  user.ID,
		Time:    amount,
		Created: created,
	}
	if _, err := x.Insert(tt); err != nil {
		return nil, err
	}
	if err := issue.loadRepo(x); err != nil {
		return nil, err
	}
	if _, err := CreateComment(&CreateCommentOptions{
		Issue:   issue,
		Repo:    issue.Repo,
		Doer:    user,
		Content: SecToTime(amount),
		Type:    CommentTypeAddTimeManual,
	}); err != nil {
		return nil, err
	}
	return tt, nil
}

// TotalTimes returns the spent time for each user by an issue
func TotalTimes(options FindTrackedTimesOptions) (map[*User]string, error) {
	trackedTimes, err := GetTrackedTimes(options)
	if err != nil {
		return nil, err
	}
	//Adding total time per user ID
	totalTimesByUser := make(map[int64]int64)
	for _, t := range trackedTimes {
		totalTimesByUser[t.UserID] += t.Time
	}

	totalTimes := make(map[*User]string)
	//Fetching User and making time human readable
	for userID, total := range totalTimesByUser {
		user, err := GetUserByID(userID)
		if err != nil {
			if IsErrUserNotExist(err) {
				continue
			}
			return nil, err
		}
		totalTimes[user] = SecToTime(total)
	}
	return totalTimes, nil
}

// DeleteIssueUserTimes deletes times for issue
func DeleteIssueUserTimes(issue *Issue, user *User) error {
	opts := FindTrackedTimesOptions{
		IssueID: issue.ID,
		UserID:  user.ID,
	}

	removedTime, err := deleteTimes(opts)

	if err := issue.loadRepo(x); err != nil {
		return err
	}
	if _, err := CreateComment(&CreateCommentOptions{
		Issue:   issue,
		Repo:    issue.Repo,
		Doer:    user,
		Content: "- " + SecToTime(removedTime),
		Type:    CommentTypeDeleteTimeManual,
	}); err != nil {
		return err
	}
	return err
}

func deleteTimes(opts FindTrackedTimesOptions) (removedTime int64, err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return
	}

	tt, err := GetTrackedTimes(opts)
	if err != nil {
		return
	}

	for _, t := range tt {
		_, err = sess.Delete(t)
		if err != nil {
			return
		}
		removedTime += t.Time
	}

	err = sess.Commit()
	return
}

// DeleteTime delete a specific Time
func DeleteTime(t *TrackedTime) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	issue, err := getIssueByID(sess, t.IssueID)
	if err != nil {
		return err
	}
	if err := issue.loadRepo(x); err != nil {
		return err
	}
	user, err := getUserByID(sess, t.UserID)
	if err != nil {
		return err
	}

	_, err = sess.Delete(t)
	if err != nil {
		return err
	}

	if _, err := CreateComment(&CreateCommentOptions{
		Issue:   issue,
		Repo:    issue.Repo,
		Doer:    user,
		Content: "- " + SecToTime(t.Time),
		Type:    CommentTypeDeleteTimeManual,
	}); err != nil {
		return err
	}

	return sess.Commit()
}

// GetTrackedTimeByID returns raw TrackedTime without loading attributes by id
func GetTrackedTimeByID(id int64) (*TrackedTime, error) {
	time := &TrackedTime{
		ID: id,
	}
	has, err := x.Get(time)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrNotExist{ID: id}
	}
	return time, nil
}
