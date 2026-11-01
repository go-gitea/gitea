// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import api "gitea.dev/modules/structs"

// swagger:response CommitComment
type swaggerResponseCommitComment struct {
	// in:body
	Body api.CommitComment `json:"body"`
}

// swagger:response CommitCommentList
type swaggerResponseCommitCommentList struct {
	// in:body
	Body []api.CommitComment `json:"body"`
}
