// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"os"

	"code.gitea.io/gitea/modules/setting"
	"github.com/go-xorm/xorm"
)

func regenerateIssueIndexer(x *xorm.Engine) error {
	_, err := os.Stat(setting.Indexer.IssuePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return os.RemoveAll(setting.Indexer.IssuePath)
}
