// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/graceful"
	repo_module "code.gitea.io/gitea/modules/repository"

	"xorm.io/xorm"
)

func regenerateGitHooks36(x *xorm.Engine) (err error) {
	return repo_module.SyncRepositoryHooks(graceful.GetManager().ShutdownContext())
}
