// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"time"

	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

// GetCodeActivityStats returns code statistics for acitivity page
func (repo *Repository) GetCodeActivityStats(fromTime time.Time, branch string) (*service.CodeActivityStats, error) {
	return common.GetCodeActivityStats(repo, fromTime, branch)
}
