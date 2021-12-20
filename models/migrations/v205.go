// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addAllowEditsFromMaintainers(x *xorm.Engine) error {
	// PullRequest represents relation between pull request and repositories.
	type PullRequest struct {
		AllowEditsFromMaintainers bool
	}

	return x.Sync2(new(PullRequest))
}
