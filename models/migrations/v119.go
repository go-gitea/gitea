// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/structs"

	"xorm.io/xorm"
)

func fixMigratedRepositoryServiceType(x *xorm.Engine) error {
	_, err := x.Exec("UPDATE repository SET original_service_type = ? WHERE original_url LIKE 'https://github.com/%'", structs.GithubService)
	return err
}
