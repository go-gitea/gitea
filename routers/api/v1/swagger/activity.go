// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// ActivityFeedsList
// swagger:response ActivityFeedsList
type swaggerActivityFeedsList struct {
	// in:body
	Body []api.Activity `json:"body"`
}
