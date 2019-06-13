// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-xorm/xorm"
)

func addCanWikiPageToc(x *xorm.Engine) error {

	type Repository struct {
		TocWikiTree bool `xorm:"NOT NULL DEFAULT true"`
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE repository SET toc_wiki_tree = ?",
		setting.Markdown.DefaultTocWikiTree); err != nil {
		return err
	}
	return nil
}
