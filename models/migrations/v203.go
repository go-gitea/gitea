// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addPrivateIssuesToRepo(x *xorm.Engine) error {
	type Repository struct {
		NumPrivateIssues       int
		NumClosedPrivateIssues int
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
