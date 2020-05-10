// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func featureChangeTargetBranch(x *xorm.Engine) error {
	type Comment struct {
		OldRef string
		NewRef string
	}

	if err := x.Sync2(new(Comment)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
