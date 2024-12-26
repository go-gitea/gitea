// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// ExploreCode
// swagger:response ExploreCode
type swaggerResponseExploreCode struct {
	// in:body
	Body api.ExploreCodeResult `json:"body"`
}
