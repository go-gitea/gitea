// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/builder"
	"xorm.io/xorm"
)

func removeLingeringIndexStatus(x *xorm.Engine) error {

	_, err := x.Exec(builder.Delete(builder.NotIn("`repo_id`", builder.Select("`id`").From("`repository`"))).From("`repo_indexer_status`"))
	return err
}
