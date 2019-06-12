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
		TocWikiFile       bool `xorm:"NOT NULL DEFAULT true"`
		TocMarkdownAlways bool `xorm:"NOT NULL DEFAULT false"`
		TocMarkdownByFlag bool `xorm:"NOT NULL DEFAULT true"`
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE repository SET toc_wiki_file = ?",
		setting.Repository.DefaultTocWikiFile); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE repository SET toc_markdown_always = ?",
		setting.Repository.DefaultTocMarkdownAlways); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE repository SET toc_markdown_by_flag = ?",
		setting.Repository.DefaultTocMarkdownByFlag); err != nil {
		return err
	}
	return nil
}
