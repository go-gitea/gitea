// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_12 //nolint

import (
	"xorm.io/xorm"
)

func FixMigratedRepositoryServiceType(x *xorm.Engine) error {
	// structs.GithubService:
	// GithubService = 2
	_, err := x.Exec("UPDATE repository SET original_service_type = ? WHERE original_url LIKE 'https://github.com/%'", 2)
	return err
}
