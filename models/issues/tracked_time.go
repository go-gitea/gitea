// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
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
func (t *TrackedTime) LoadAttributes(ctx context.Context) (err error) {
	// Load the issue
	if t.Issue == nil {
		t.Issue, err = GetIssueByID(ctx, t.IssueID)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return err
		}
	}
	// Now load the repo for the issue (which we may have just loaded)
	if t.Issue != nil {
		err = t.Issue.LoadRepo(ctx)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return err
		}
	}
	// Load the user
	if t.User == nil {
		t.User, err = user_model.GetUserByID(ctx, t.UserID)
		if err != nil {
			if !errors.Is(err, util.ErrNotExist) {
				return err
			}
			t.User = user_model.NewGhostUser()
		}
	}
	return nil
}

// LoadAttributes load Issue, User
func (tl TrackedTimeList) LoadAttributes(ctx context.Context) error {
	for _, t := range tl {
		if err := t.LoadAttributes(ctx); err != nil {
			return err
		}
	}
	return nil
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
func (opts *FindTrackedTimesOptions) ToConds() builder.Cond {
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

func (opts *FindTrackedTimesOptions) ToJoins() []db.JoinFunc {
	if opts.RepositoryID > 0 || opts.MilestoneID > 0 {
		return []db.JoinFunc{
			func(e db.Engine) error {
				e.Join("INNER", "issue", "issue.id = tracked_time.issue_id")
				return nil
			},
		}
	}
	return nil
}

// toSession will convert the given options to a xorm Session by using the conditions from toCond and joining with issue table if required
func (opts *FindTrackedTimesOptions) toSession(e db.Engine) db.Engine {
	sess := e
	if opts.RepositoryID > 0 || opts.MilestoneID > 0 {
		sess = e.Join("INNER", "issue", "issue.id = tracked_time.issue_id")
	}

	sess = sess.Where(opts.ToConds())

	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, opts)
	}

	return sess
}

// GetTrackedTimes returns all tracked times that fit to the given options.
func GetTrackedTimes(ctx context.Context, options *FindTrackedTimesOptions) (trackedTimes TrackedTimeList, err error) {
	err = options.toSession(db.GetEngine(ctx)).Find(&trackedTimes)
	return trackedTimes, err
}

// CountTrackedTimes returns count of tracked times that fit to the given options.
func CountTrackedTimes(ctx context.Context, opts *FindTrackedTimesOptions) (int64, error) {
	sess := db.GetEngine(ctx).Where(opts.ToConds())
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
func AddTime(ctx context.Context, user *user_model.User, issue *Issue, amount int64, created time.Time) (*TrackedTime, error) {
	ctx, committer, err := db.TxContext(ctx)
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

	if _, err := CreateComment(ctx, &CreateCommentOptions{
		Issue: issue,
		Repo:  issue.Repo,
		Doer:  user,
		// Content before v1.21 did store the formatted string instead of seconds,
		// so use "|" as delimiter to mark the new format
		Content: fmt.Sprintf("|%d", amount),
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

// TotalTimesForEachUser returns the spent time in seconds for each user by an issue
func TotalTimesForEachUser(ctx context.Context, options *FindTrackedTimesOptions) (map[*user_model.User]int64, error) {
	trackedTimes, err := GetTrackedTimes(ctx, options)
	if err != nil {
		return nil, err
	}
	// Adding total time per user ID
	totalTimesByUser := make(map[int64]int64)
	for _, t := range trackedTimes {
		totalTimesByUser[t.UserID] += t.Time
	}

	totalTimes := make(map[*user_model.User]int64)
	// Fetching User and making time human readable
	for userID, total := range totalTimesByUser {
		user, err := user_model.GetUserByID(ctx, userID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				continue
			}
			return nil, err
		}
		totalTimes[user] = total
	}
	return totalTimes, nil
}

// DeleteIssueUserTimes deletes times for issue
func DeleteIssueUserTimes(ctx context.Context, issue *Issue, user *user_model.User) error {
	ctx, committer, err := db.TxContext(ctx)
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
		return db.ErrNotExist{Resource: "tracked_time"}
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}
	if _, err := CreateComment(ctx, &CreateCommentOptions{
		Issue: issue,
		Repo:  issue.Repo,
		Doer:  user,
		// Content before v1.21 did store the formatted string instead of seconds,
		// so use "|" as delimiter to mark the new format
		Content: fmt.Sprintf("|%d", removedTime),
		Type:    CommentTypeDeleteTimeManual,
	}); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteTime delete a specific Time
func DeleteTime(ctx context.Context, t *TrackedTime) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := t.LoadAttributes(ctx); err != nil {
		return err
	}

	if err := deleteTime(ctx, t); err != nil {
		return err
	}

	if _, err := CreateComment(ctx, &CreateCommentOptions{
		Issue: t.Issue,
		Repo:  t.Issue.Repo,
		Doer:  t.User,
		// Content before v1.21 did store the formatted string instead of seconds,
		// so use "|" as delimiter to mark the new format
		Content: fmt.Sprintf("|%d", t.Time),
		Type:    CommentTypeDeleteTimeManual,
	}); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteTimes(ctx context.Context, opts FindTrackedTimesOptions) (removedTime int64, err error) {
	removedTime, err = GetTrackedSeconds(ctx, opts)
	if err != nil || removedTime == 0 {
		return removedTime, err
	}

	_, err = opts.toSession(db.GetEngine(ctx)).Table("tracked_time").Cols("deleted").Update(&TrackedTime{Deleted: true})
	return removedTime, err
}

func deleteTime(ctx context.Context, t *TrackedTime) error {
	if t.Deleted {
		return db.ErrNotExist{Resource: "tracked_time", ID: t.ID}
	}
	t.Deleted = true
	_, err := db.GetEngine(ctx).ID(t.ID).Cols("deleted").Update(t)
	return err
}

// GetTrackedTimeByID returns raw TrackedTime without loading attributes by id
func GetTrackedTimeByID(ctx context.Context, id int64) (*TrackedTime, error) {
	time := new(TrackedTime)
	has, err := db.GetEngine(ctx).ID(id).Get(time)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, db.ErrNotExist{Resource: "tracked_time", ID: id}
	}
	return time, nil
}

// GetIssueTotalTrackedTime returns the total tracked time for issues by given conditions.
func GetIssueTotalTrackedTime(ctx context.Context, opts *IssuesOptions, isClosed optional.Option[bool]) (int64, error) {
	if len(opts.IssueIDs) <= MaxQueryParameters {
		return getIssueTotalTrackedTimeChunk(ctx, opts, isClosed, opts.IssueIDs)
	}

	// If too long a list of IDs is provided,
	// we get the statistics in smaller chunks and get accumulates
	var accum int64
	for i := 0; i < len(opts.IssueIDs); {
		chunk := i + MaxQueryParameters
		if chunk > len(opts.IssueIDs) {
			chunk = len(opts.IssueIDs)
		}
		time, err := getIssueTotalTrackedTimeChunk(ctx, opts, isClosed, opts.IssueIDs[i:chunk])
		if err != nil {
			return 0, err
		}
		accum += time
		i = chunk
	}
	return accum, nil
}

func getIssueTotalTrackedTimeChunk(ctx context.Context, opts *IssuesOptions, isClosed optional.Option[bool], issueIDs []int64) (int64, error) {
	sumSession := func(opts *IssuesOptions, issueIDs []int64) *xorm.Session {
		sess := db.GetEngine(ctx).
			Table("tracked_time").
			Where("tracked_time.deleted = ?", false).
			Join("INNER", "issue", "tracked_time.issue_id = issue.id")

		return applyIssuesOptions(sess, opts, issueIDs)
	}

	type trackedTime struct {
		Time int64
	}

	session := sumSession(opts, issueIDs)
	if isClosed.Has() {
		session = session.And("issue.is_closed = ?", isClosed.Value())
	}
	return session.SumInt(new(trackedTime), "tracked_time.time")
}
