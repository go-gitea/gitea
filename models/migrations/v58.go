// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addLabelsDescriptions(x *xorm.Engine) error {
	type Label struct {
		Description string
	}

	if err := x.Sync2(new(Label)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
