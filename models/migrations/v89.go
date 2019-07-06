// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-xorm/xorm"
)

func addCanTocOnWikiAndMarkdown(x *xorm.Engine) error {

	type Repository struct {
		TocWikiFile     bool `xorm:"NOT NULL DEFAULT true"`
		TocMarkupAlways bool `xorm:"NOT NULL DEFAULT false"`
		TocMarkupByFlag bool `xorm:"NOT NULL DEFAULT true"`
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE repository SET toc_wiki_file = ?",
		setting.Markdown.DefaultTocWikiFile); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE repository SET toc_markup_always = ?",
		setting.Markdown.DefaultTocMarkupAlways); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE repository SET toc_markup_by_flag = ?",
		setting.Markdown.DefaultTocMarkupByFlag); err != nil {
		return err
	}
	return nil
}
