// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ActionList defines a list of actions
type ActionList []*Action

func (actions ActionList) getUserIDs() []int64 {
	return container.FilterSlice(actions, func(action *Action) (int64, bool) {
		return action.ActUserID, true
	})
}

func (actions ActionList) LoadActUsers(ctx context.Context) (map[int64]*user_model.User, error) {
	if len(actions) == 0 {
		return nil, nil //nolint:nilnil // returns nil when there are no actions
	}

	userIDs := actions.getUserIDs()
	userMaps := make(map[int64]*user_model.User, len(userIDs))
	err := db.GetEngine(ctx).
		In("id", userIDs).
		Find(&userMaps)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	for _, action := range actions {
		action.ActUser = userMaps[action.ActUserID]
	}
	return userMaps, nil
}

func (actions ActionList) getRepoIDs() []int64 {
	return container.FilterSlice(actions, func(action *Action) (int64, bool) {
		return action.RepoID, true
	})
}

func (actions ActionList) LoadRepositories(ctx context.Context) error {
	if len(actions) == 0 {
		return nil
	}

	repoIDs := actions.getRepoIDs()
	repoMaps := make(map[int64]*repo_model.Repository, len(repoIDs))
	err := db.GetEngine(ctx).In("id", repoIDs).Find(&repoMaps)
	if err != nil {
		return fmt.Errorf("find repository: %w", err)
	}
	for _, action := range actions {
		action.Repo = repoMaps[action.RepoID]
	}
	repos := repo_model.RepositoryList(util.ValuesOfMap(repoMaps))
	return repos.LoadUnits(ctx)
}

func (actions ActionList) loadRepoOwner(ctx context.Context, userMap map[int64]*user_model.User) (err error) {
	if userMap == nil {
		userMap = make(map[int64]*user_model.User)
	}

	missingUserIDs := container.FilterSlice(actions, func(action *Action) (int64, bool) {
		if action.Repo == nil {
			return 0, false
		}
		_, alreadyLoaded := userMap[action.Repo.OwnerID]
		return action.Repo.OwnerID, !alreadyLoaded
	})
	if len(missingUserIDs) == 0 {
		return nil
	}

	if err := db.GetEngine(ctx).
		In("id", missingUserIDs).
		Find(&userMap); err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	for _, action := range actions {
		if action.Repo != nil {
			action.Repo.Owner = userMap[action.Repo.OwnerID]
		}
	}

	return nil
}

// LoadAttributes loads all attributes
func (actions ActionList) LoadAttributes(ctx context.Context) error {
	// the load sequence cannot be changed because of the dependencies
	userMap, err := actions.LoadActUsers(ctx)
	if err != nil {
		return err
	}
	if err := actions.LoadRepositories(ctx); err != nil {
		return err
	}
	if err := actions.loadRepoOwner(ctx, userMap); err != nil {
		return err
	}
	if err := actions.LoadIssues(ctx); err != nil {
		return err
	}
	return actions.LoadComments(ctx)
}

func (actions ActionList) LoadComments(ctx context.Context) error {
	if len(actions) == 0 {
		return nil
	}

	commentIDs := make([]int64, 0, len(actions))
	for _, action := range actions {
		if action.CommentID > 0 {
			commentIDs = append(commentIDs, action.CommentID)
		}
	}
	if len(commentIDs) == 0 {
		return nil
	}

	commentsMap := make(map[int64]*issues_model.Comment, len(commentIDs))
	if err := db.GetEngine(ctx).In("id", commentIDs).Find(&commentsMap); err != nil {
		return fmt.Errorf("find comment: %w", err)
	}

	for _, action := range actions {
		if action.CommentID > 0 {
			action.Comment = commentsMap[action.CommentID]
			if action.Comment != nil {
				action.Comment.Issue = action.Issue
			}
		}
	}
	return nil
}

func (actions ActionList) LoadIssues(ctx context.Context) error {
	if len(actions) == 0 {
		return nil
	}

	conditions := builder.NewCond()
	issueNum := 0
	for _, action := range actions {
		if action.IsIssueEvent() {
			infos := action.GetIssueInfos()
			if len(infos) == 0 {
				continue
			}
			index, _ := strconv.ParseInt(infos[0], 10, 64)
			if index > 0 {
				conditions = conditions.Or(builder.Eq{
					"repo_id": action.RepoID,
					"`index`": index,
				})
				issueNum++
			}
		}
	}
	if !conditions.IsValid() {
		return nil
	}

	issuesMap := make(map[string]*issues_model.Issue, issueNum)
	issues := make([]*issues_model.Issue, 0, issueNum)
	if err := db.GetEngine(ctx).Where(conditions).Find(&issues); err != nil {
		return fmt.Errorf("find issue: %w", err)
	}
	for _, issue := range issues {
		issuesMap[fmt.Sprintf("%d-%d", issue.RepoID, issue.Index)] = issue
	}

	for _, action := range actions {
		if !action.IsIssueEvent() {
			continue
		}
		if index := action.getIssueIndex(); index > 0 {
			if issue, ok := issuesMap[fmt.Sprintf("%d-%d", action.RepoID, index)]; ok {
				action.Issue = issue
				action.Issue.Repo = action.Repo
			}
		}
	}
	return nil
}

// GetFeeds returns actions according to the provided options
func GetFeeds(ctx context.Context, opts GetFeedsOptions) (ActionList, int64, error) {
	if opts.RequestedUser == nil && opts.RequestedTeam == nil && opts.RequestedRepo == nil {
		return nil, 0, errors.New("need at least one of these filters: RequestedUser, RequestedTeam, RequestedRepo")
	}

	var err error
	var cond builder.Cond
	// if the actor is the requested user or is an administrator, we can skip the ActivityQueryCondition
	if opts.Actor != nil && opts.RequestedUser != nil && (opts.Actor.IsAdmin || opts.Actor.ID == opts.RequestedUser.ID) {
		cond = builder.Eq{
			"user_id": opts.RequestedUser.ID,
		}.And(
			FeedDateCond(opts),
		)

		if !opts.IncludeDeleted {
			cond = cond.And(builder.Eq{"is_deleted": false})
		}

		if !opts.IncludePrivate {
			cond = cond.And(builder.Eq{"is_private": false})
		}
		if opts.OnlyPerformedBy {
			cond = cond.And(builder.Eq{"act_user_id": opts.RequestedUser.ID})
		}
	} else {
		cond, err = ActivityQueryCondition(ctx, opts)
		if err != nil {
			return nil, 0, err
		}
	}

	actions := make([]*Action, 0, opts.PageSize)
	var count int64
	opts.SetDefaultValues()

	if opts.Page < 10 { // TODO: why it's 10 but other values? It's an experience value.
		sess := db.GetEngine(ctx).Where(cond)
		db.SetSessionPagination(sess, &opts)

		if opts.DontCount {
			err = sess.Desc("`action`.created_unix").Find(&actions)
		} else {
			count, err = sess.Desc("`action`.created_unix").FindAndCount(&actions)
		}
		if err != nil {
			return nil, 0, fmt.Errorf("FindAndCount: %w", err)
		}
	} else {
		// First, only query which IDs are necessary, and only then query all actions to speed up the overall query
		sess := db.GetEngine(ctx).Where(cond).Select("`action`.id")
		db.SetSessionPagination(sess, &opts)

		actionIDs := make([]int64, 0, opts.PageSize)
		if err := sess.Table("action").Desc("`action`.created_unix").Find(&actionIDs); err != nil {
			return nil, 0, fmt.Errorf("Find(actionsIDs): %w", err)
		}

		if !opts.DontCount {
			count, err = db.GetEngine(ctx).Where(cond).
				Table("action").
				Cols("`action`.id").Count()
			if err != nil {
				return nil, 0, fmt.Errorf("Count: %w", err)
			}
		}

		if err := db.GetEngine(ctx).In("`action`.id", actionIDs).Desc("`action`.created_unix").Find(&actions); err != nil {
			return nil, 0, fmt.Errorf("Find: %w", err)
		}
	}

	if err := ActionList(actions).LoadAttributes(ctx); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %w", err)
	}

	return actions, count, nil
}

// HiddenActivityRollup is the per-day count of a user's actions that the viewer
// cannot see. Time is the day's midnight (server timezone) so the rollup sorts
// below the day's visible actions; DisplayTime is the day's noon, which keeps
// browser-local date rendering on the right day for viewers within 12h of the
// server timezone while revealing nothing beyond the day itself.
type HiddenActivityRollup struct {
	Time        timeutil.TimeStamp
	DisplayTime timeutil.TimeStamp
	Count       int64
}

// dayStartOf returns the start of ts's day in the server timezone; when a DST
// transition removes midnight, the day starts right after the gap instead of
// resolving into the previous day
func dayStartOf(ts timeutil.TimeStamp) timeutil.TimeStamp {
	day := ts.FormatInLocation("2006-01-02", setting.DefaultUILocation)
	tt := ts.AsTimeInLocation(setting.DefaultUILocation)
	start := time.Date(tt.Year(), tt.Month(), tt.Day(), 0, 0, 0, 0, setting.DefaultUILocation)
	for timeutil.TimeStamp(start.Unix()).FormatInLocation("2006-01-02", setting.DefaultUILocation) != day {
		start = start.Add(time.Hour)
	}
	return timeutil.TimeStamp(start.Unix())
}

func dayBoundsOf(ts timeutil.TimeStamp) (dayStart, nextDayStart timeutil.TimeStamp) {
	dayStart = dayStartOf(ts)
	// a day is 23-25h long, so 26h past its start is always inside the next day
	return dayStart, dayStartOf(dayStart.Add(26 * 60 * 60))
}

// FindHiddenUserActivityRollups groups the requested user's own actions that the
// actor cannot see into per-day rollups (newest first, days per the server's
// default timezone like FeedDateCond), within the time span covered by the given
// feed page. Page spans are snapped to whole days so that a page boundary never
// reveals how hidden actions interleave with visible ones inside a day: a day's
// hidden actions all surface on the page showing the day's oldest visible
// action, and days without any visible action surface on the page whose span
// contains them. Consecutive pages decide their shared edge day with the same
// query, so per-page rollups still sum up to the feed-wide totals. opts must be
// the options the visible page was fetched with, pageActions the actions it
// returned.
func FindHiddenUserActivityRollups(ctx context.Context, opts GetFeedsOptions, pageActions ActionList) ([]*HiddenActivityRollup, error) {
	if opts.RequestedUser == nil || opts.PageSize <= 0 {
		return nil, errors.New("need RequestedUser and a positive PageSize")
	}
	opts.SetDefaultValues() // clamp PageSize the same way GetFeeds does

	visibleCond, err := ActivityQueryCondition(ctx, opts)
	if err != nil {
		return nil, err
	}

	cond := builder.Eq{"user_id": opts.RequestedUser.ID, "act_user_id": opts.RequestedUser.ID}.
		And(FeedDateCond(opts)).
		And(builder.Not{visibleCond})

	// reports whether a visible action exists in [dayStart, before), i.e. whether
	// the day of `before` continues on later feed pages, and if so the oldest such
	// action; both pages sharing an edge day call this with identical bounds and
	// so agree on which one of them rolls the day up
	oldestVisibleBefore := func(dayStart, before timeutil.TimeStamp) (timeutil.TimeStamp, bool, error) {
		var oldest int64
		has, err := db.GetEngine(ctx).Table("action").Where(visibleCond).
			And(builder.Gte{"`action`.created_unix": dayStart}).
			And(builder.Lt{"`action`.created_unix": before}).
			Cols("created_unix").Asc("created_unix").Limit(1).Get(&oldest)
		return timeutil.TimeStamp(oldest), has, err
	}

	pageFull := len(pageActions) == opts.PageSize // a short page is the last one

	if opts.Page > 1 {
		var boundary int64
		has, err := db.GetEngine(ctx).Table("action").Where(visibleCond).
			Cols("created_unix").Desc("created_unix").
			Limit(1, (opts.Page-1)*opts.PageSize-1).Get(&boundary)
		if err != nil {
			return nil, err
		}
		if !has { // the page is beyond the visible feed, its span is already covered
			return nil, nil
		}
		dayStart, nextDay := dayBoundsOf(timeutil.TimeStamp(boundary))
		// the boundary day rolls up here only when the day's oldest visible
		// action also falls on this page; otherwise the previous page (day ends
		// at the boundary) or a later page (day continues past this one) owns it
		oldest, dayContinues, err := oldestVisibleBefore(dayStart, timeutil.TimeStamp(boundary))
		if err != nil {
			return nil, err
		}
		if dayContinues && (!pageFull || oldest >= pageActions[len(pageActions)-1].CreatedUnix) {
			cond = cond.And(builder.Lt{"`action`.created_unix": nextDay})
		} else {
			cond = cond.And(builder.Lt{"`action`.created_unix": dayStart})
		}
	}
	if pageFull {
		oldestOnPage := pageActions[len(pageActions)-1].CreatedUnix
		dayStart, nextDay := dayBoundsOf(oldestOnPage)
		// mirror image of the boundary-day decision above, for this page's own
		// oldest day against the next page
		_, dayContinues, err := oldestVisibleBefore(dayStart, oldestOnPage)
		if err != nil {
			return nil, err
		}
		if dayContinues {
			cond = cond.And(builder.Gte{"`action`.created_unix": nextDay})
		} else {
			cond = cond.And(builder.Gte{"`action`.created_unix": dayStart})
		}
	}

	groupBy := "created_unix / 900 * 900"
	groupByName := "timestamp" // mssql does not allow grouping by alias
	switch {
	case setting.Database.Type.IsMySQL():
		groupBy = "created_unix DIV 900 * 900"
	case setting.Database.Type.IsMSSQL():
		groupByName = groupBy
	}

	// aggregate like the heatmap does: 15-minute buckets bound the result size
	// regardless of how many hidden actions exist, and cannot straddle a day
	// boundary because timezone offsets are multiples of 15 minutes
	buckets := make([]*UserHeatmapData, 0, 8)
	if err := db.GetEngine(ctx).Table("action").Where(cond).
		Select(groupBy + " AS timestamp, count(user_id) AS contributions").
		GroupBy(groupByName).OrderBy("timestamp DESC").Find(&buckets); err != nil {
		return nil, err
	}

	rollups := make([]*HiddenActivityRollup, 0, 8)
	lastDay := ""
	for _, bucket := range buckets {
		if day := bucket.Timestamp.FormatInLocation("2006-01-02", setting.DefaultUILocation); day != lastDay {
			dayStart := dayStartOf(bucket.Timestamp)
			rollups = append(rollups, &HiddenActivityRollup{
				Time:        dayStart,
				DisplayTime: dayStart.Add(12 * 60 * 60),
			})
			lastDay = day
		}
		rollups[len(rollups)-1].Count += bucket.Contributions
	}
	return rollups, nil
}
