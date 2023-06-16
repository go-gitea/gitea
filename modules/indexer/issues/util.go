// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

func getIssueIndexerData(ctx context.Context, issueID int64) (*internal.IndexerData, bool, error) {
	// TODO
	return nil, false, nil
}
