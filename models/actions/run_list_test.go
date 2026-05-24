// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

// GetRunBranches reads from the `branch` table, which is owned by models/git.
// Importing models/git here would create an import cycle (git -> perm/access -> actions),
// and without that import xorm does not register the table for this test binary.
// Coverage for GetRunBranches lives in tests/integration/actions_list_filter_test.go,
// which exercises it against the full schema.
