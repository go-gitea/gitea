// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import "xorm.io/xorm"

func AddDefaultWikiBranch(x *xorm.Engine) error {
	type Repository struct {
		ID                int64
		DefaultWikiBranch string
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices:    true,
		IgnoreConstrains: true,
	}, &Repository{}); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE `repository` SET default_wiki_branch = 'master' WHERE (default_wiki_branch IS NULL) OR (default_wiki_branch = '')")
	return err
}
