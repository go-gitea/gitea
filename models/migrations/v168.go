// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func recreateUserTableToFixDefaultValues(_ *xorm.Engine) error {
	return nil
}
