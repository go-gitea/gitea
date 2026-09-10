// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	module_git "gitea.dev/modules/git"
)

// IsRefProtected checks whether a branch or tag ref is protected.
func IsRefProtected(ctx context.Context, repoID int64, ref module_git.RefName) (bool, error) {
	if ref.IsBranch() {
		return IsBranchProtected(ctx, repoID, ref.ShortName())
	}
	if !ref.IsTag() {
		return false, nil
	}

	protectedTags, err := GetProtectedTags(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("get protected tags: %w", err)
	}
	for _, protectedTag := range protectedTags {
		if err := protectedTag.EnsureCompiledPattern(); err != nil {
			return false, fmt.Errorf("compile protected tag pattern %q: %w", protectedTag.NamePattern, err)
		}
		if protectedTag.matchString(ref.ShortName()) {
			return true, nil
		}
	}

	return false, nil
}
