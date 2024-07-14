// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// Project
// swagger:response Project
type swaggerResponseProject struct {
	// in:body
	Body api.Project `json:"body"`
}
