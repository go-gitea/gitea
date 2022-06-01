// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// TrackedTime represents a time that was spent for a specific issue.
type TrackedTime struct {
	ID          int64            `xorm:"pk autoincr"`
	IssueID     int64            `xorm:"INDEX"`
	Issue       *Issue           `xorm:"-"`
	UserID      int64            `xorm:"INDEX"`
	User        *user_model.User `xorm:"-"`
	Created     time.Time        `xorm:"-"`
	CreatedUnix int64            `xorm:"created"`
	Time        int64            `xorm:"NOT NULL"`
	Deleted     bool             `xorm:"NOT NULL DEFAULT false"`
}

func init() {
	db.RegisterModel(new(TrackedTime))
}

// TrackedTimeList is a List of TrackedTime's
type TrackedTimeList []*TrackedTime

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (t *TrackedTime) AfterLoad() {
	t.Created = time.Unix(t.CreatedUnix, 0).In(setting.DefaultUILocation)
}

// LoadAttributes load Issue, User
func (t *TrackedTime) LoadAttributes() (err error) {
	return t.loadAttributes(db.DefaultContext)
}

func (t *TrackedTime) loadAttributes(ctx context.Context) (err error) {
	if t.Issue == nil {
		t.Issue, err = getIssueByID(ctx, t.IssueID)
		if err != nil {
			return
		}
		err = t.Issue.LoadRepo(ctx)
		if err != nil {
			return
		}
	}
	if t.User == nil {
		t.User, err = user_model.GetUserByIDCtx(ctx, t.UserID)
		if err != nil {
			return
		}
	}
	return
}

// LoadAttributes load Issue, User
func (tl TrackedTimeList) LoadAttributes() (err error) {
	for _, t := range tl {
		if err = t.LoadAttributes(); err != nil {
			return err
		}
	}
	return
}

// FindTrackedTimesOptions represent the filters for tracked times. If an ID is 0 it will be ignored.
type FindTrackedTimesOptions struct {
	db.ListOptions
	IssueID           int64
	UserID            int64
	RepositoryID      int64
	MilestoneID       int64
	CreatedAfterUnix  int64
	CreatedBeforeUnix int64
}

// toCond will convert each condition into a xorm-Cond
func (opts *FindTrackedTimesOptions) toCond() builder.Cond {
	cond := builder.NewCond().And(builder.Eq{"tracked_time.deleted": false})
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
	if opts.CreatedAfterUnix != 0 {
		cond = cond.And(builder.Gte{"tracked_time.created_unix": opts.CreatedAfterUnix})
	}
	if opts.CreatedBeforeUnix != 0 {
		cond = cond.And(builder.Lte{"tracked_time.created_unix": opts.CreatedBeforeUnix})
	}
	return cond
}

// toSession will convert the given options to a xorm Session by using the conditions from toCond and joining with issue table if required
func (opts *FindTrackedTimesOptions) toSession(e db.Engine) db.Engine {
	sess := e
	if opts.RepositoryID > 0 || opts.MilestoneID > 0 {
		sess = e.Join("INNER", "issue", "issue.id = tracked_time.issue_id")
	}

	sess = sess.Where(opts.toCond())

	if opts.Page != 0 {
		sess = db.SetEnginePagination(sess, opts)
	}

	return sess
}

// GetTrackedTimes returns all tracked times that fit to the given options.
func GetTrackedTimes(ctx context.Context, options *FindTrackedTimesOptions) (trackedTimes TrackedTimeList, err error) {
	err = options.toSession(db.GetEngine(ctx)).Find(&trackedTimes)
	return
}

// CountTrackedTimes returns count of tracked times that fit to the given options.
func CountTrackedTimes(opts *FindTrackedTimesOptions) (int64, error) {
	sess := db.GetEngine(db.DefaultContext).Where(opts.toCond())
	if opts.RepositoryID > 0 || opts.MilestoneID > 0 {
		sess = sess.Join("INNER", "issue", "issue.id = tracked_time.issue_id")
	}
	return sess.Count(&TrackedTime{})
}

// GetTrackedSeconds return sum of seconds
func GetTrackedSeconds(ctx context.Context, opts FindTrackedTimesOptions) (trackedSeconds int64, err error) {
	return opts.toSession(db.GetEngine(ctx)).SumInt(&TrackedTime{}, "time")
}

// AddTime will add the given time (in seconds) to the issue
func AddTime(user *user_model.User, issue *Issue, amount int64, created time.Time) (*TrackedTime, error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	t, err := addTime(ctx, user, issue, amount, created)
	if err != nil {
		return nil, err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	if _, err := CreateCommentCtx(ctx, &CreateCommentOptions{
		Issue:   issue,
		Repo:    issue.Repo,
		Doer:    user,
		Content: util.SecToTime(amount),
		Type:    CommentTypeAddTimeManual,
		TimeID:  t.ID,
	}); err != nil {
		return nil, err
	}

	return t, committer.Commit()
}

func addTime(ctx context.Context, user *user_model.User, issue *Issue, amount int64, created time.Time) (*TrackedTime, error) {
	if created.IsZero() {
		created = time.Now()
	}
	tt := &TrackedTime{
		IssueID: issue.ID,
		UserID:  user.ID,
		Time:    amount,
		Created: created,
	}
	return tt, db.Insert(ctx, tt)
}

// TotalTimes returns the spent time for each user by an issue
func TotalTimes(options *FindTrackedTimesOptions) (map[*user_model.User]string, error) {
	trackedTimes, err := GetTrackedTimes(db.DefaultContext, options)
	if err != nil {
		return nil, err
	}
	// Adding total time per user ID
	totalTimesByUser := make(map[int64]int64)
	for _, t := range trackedTimes {
		totalTimesByUser[t.UserID] += t.Time
	}

	totalTimes := make(map[*user_model.User]string)
	// Fetching User and making time human readable
	for userID, total := range totalTimesByUser {
		user, err := user_model.GetUserByID(userID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				continue
			}
			return nil, err
		}
		totalTimes[user] = util.SecToTime(total)
	}
	return totalTimes, nil
}

// DeleteIssueUserTimes deletes times for issue
func DeleteIssueUserTimes(issue *Issue, user *user_model.User) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	opts := FindTrackedTimesOptions{
		IssueID: issue.ID,
		UserID:  user.ID,
	}

	removedTime, err := deleteTimes(ctx, opts)
	if err != nil {
		return err
	}
	if removedTime == 0 {
		return db.ErrNotExist{}
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}
	if _, err := CreateCommentCtx(ctx, &CreateCommentOptions{
		Issue:   issue,
		Repo:    issue.Repo,
		Doer:    user,
		Content: "- " + util.SecToTime(removedTime),
		Type:    CommentTypeDeleteTimeManual,
	}); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteTime delete a specific Time
func DeleteTime(t *TrackedTime) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := t.loadAttributes(ctx); err != nil {
		return err
	}

	if err := deleteTime(ctx, t); err != nil {
		return err
	}

	if _, err := CreateCommentCtx(ctx, &CreateCommentOptions{
		Issue:   t.Issue,
		Repo:    t.Issue.Repo,
		Doer:    t.User,
		Content: "- " + util.SecToTime(t.Time),
		Type:    CommentTypeDeleteTimeManual,
	}); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteTimes(ctx context.Context, opts FindTrackedTimesOptions) (removedTime int64, err error) {
	removedTime, err = GetTrackedSeconds(ctx, opts)
	if err != nil || removedTime == 0 {
		return
	}

	_, err = opts.toSession(db.GetEngine(ctx)).Table("tracked_time").Cols("deleted").Update(&TrackedTime{Deleted: true})
	return
}

func deleteTime(ctx context.Context, t *TrackedTime) error {
	if t.Deleted {
		return db.ErrNotExist{ID: t.ID}
	}
	t.Deleted = true
	_, err := db.GetEngine(ctx).ID(t.ID).Cols("deleted").Update(t)
	return err
}

// GetTrackedTimeByID returns raw TrackedTime without loading attributes by id
func GetTrackedTimeByID(id int64) (*TrackedTime, error) {
	time := new(TrackedTime)
	has, err := db.GetEngine(db.DefaultContext).ID(id).Get(time)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, db.ErrNotExist{ID: id}
	}
	return time, nil
}
