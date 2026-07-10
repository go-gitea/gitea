// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import api "gitea.dev/modules/structs"

// Group
// swagger:response Group
type swaggerResponseGroup struct {
	// in:body
	Body api.Group `json:"body"`
}

// GroupList
// swagger:response GroupList
type swaggerResponseGroupList struct {
	// in:body
	Body []api.Group `json:"body"`
}
