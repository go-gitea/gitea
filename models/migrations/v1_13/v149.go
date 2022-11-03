// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_13 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddCreatedAndUpdatedToMilestones(x *xorm.Engine) error {
	type Milestone struct {
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := x.Sync2(new(Milestone)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
