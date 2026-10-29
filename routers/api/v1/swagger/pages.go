// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import api "gitea.dev/modules/structs"

// PagesContentCommit
// swagger:response PagesContentCommit
type swaggerResponsePagesContentCommit struct {
	// in:body
	Body api.PagesContentCommit `json:"body"`
}
