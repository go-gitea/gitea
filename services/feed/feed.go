// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/activities"
	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
)

// GetFeeds returns actions according to the provided options
func GetFeeds(ctx context.Context, opts activities_model.GetFeedsOptions) (activities.ActionList, int64, error) {
	if opts.RequestedUser == nil && opts.RequestedTeam == nil && opts.RequestedRepo == nil {
		return nil, 0, fmt.Errorf("need at least one of these filters: RequestedUser, RequestedTeam, RequestedRepo")
	}

	cond, err := activities_model.ActivityQueryCondition(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	actions := make([]*activities_model.Action, 0, opts.PageSize)
	var count int64
	opts.SetDefaultValues()

	if opts.Page < 10 { // TODO: why it's 10 but other values? It's an experience value.
		sess := db.GetEngine(ctx).Where(cond)
		sess = db.SetSessionPagination(sess, &opts)

		count, err = sess.Desc("`action`.created_unix").FindAndCount(&actions)
		if err != nil {
			return nil, 0, fmt.Errorf("FindAndCount: %w", err)
		}
	} else {
		// First, only query which IDs are necessary, and only then query all actions to speed up the overall query
		sess := db.GetEngine(ctx).Where(cond).Select("`action`.id")
		sess = db.SetSessionPagination(sess, &opts)

		actionIDs := make([]int64, 0, opts.PageSize)
		if err := sess.Table("action").Desc("`action`.created_unix").Find(&actionIDs); err != nil {
			return nil, 0, fmt.Errorf("Find(actionsIDs): %w", err)
		}

		count, err = db.GetEngine(ctx).Where(cond).
			Table("action").
			Cols("`action`.id").Count()
		if err != nil {
			return nil, 0, fmt.Errorf("Count: %w", err)
		}

		if err := db.GetEngine(ctx).In("`action`.id", actionIDs).Desc("`action`.created_unix").Find(&actions); err != nil {
			return nil, 0, fmt.Errorf("Find: %w", err)
		}
	}

	if err := activities_model.ActionList(actions).LoadAttributes(ctx); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %w", err)
	}

	return actions, count, nil
}
