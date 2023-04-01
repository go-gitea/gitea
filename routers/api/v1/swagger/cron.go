// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// CronList
// swagger:response CronList
type swaggerResponseCronList struct {
	// in:body
	Body []api.Cron `json:"body"`
}
