// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// CodeSearchResults
// swagger:response CodeSearchResults
type swaggerResponseCodeSearchResults struct {
	// in:body
	Body api.CodeSearchResults `json:"body"`
}
