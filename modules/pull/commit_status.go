// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import "code.gitea.io/gitea/models"

// IsCommitStatusContextSuccess returns true if all required status check contexts succeed.
func IsCommitStatusContextSuccess(commitStatuses []*models.CommitStatus, requiredContexts []string) bool {
	for _, ctx := range requiredContexts {
		var found bool
		for _, commitStatus := range commitStatuses {
			if commitStatus.Context == ctx {
				if commitStatus.State != models.CommitStatusSuccess {
					return false
				}

				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
