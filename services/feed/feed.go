// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"context"

	activities_model "code.gitea.io/gitea/models/activities"
)

// GetFeeds returns actions according to the provided options
func GetFeeds(ctx context.Context, opts activities_model.GetFeedsOptions) (activities_model.ActionList, int64, error) {
	return activities_model.GetFeeds(ctx, opts)
}
