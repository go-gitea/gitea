// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"xorm.io/xorm"
)

func recalculateStars(x *xorm.Engine) (err error) {
	// because of issue https://github.com/go-gitea/gitea/issues/11949,
	// recalculate Stars number for all users to fully fix it.

	return models.DoctorUserStarNum()
}
