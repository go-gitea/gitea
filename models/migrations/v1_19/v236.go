// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_19 //nolint

import (
	"xorm.io/xorm"
)

func AddManuallyMergePullConfirmedToPullRequest(x *xorm.Engine) error {
	type PullRequest struct {
		ID int64 `xorm:"pk autoincr"`

		ManuallyMergePullConfirmed bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(PullRequest))
}
