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

// PagesVersionList
// swagger:response PagesVersionList
type swaggerResponsePagesVersionList struct {
	// in:body
	Body []api.PagesVersion `json:"body"`
}

// PagesDeploymentResponse
// swagger:response PagesDeploymentResponse
type swaggerResponsePagesDeploymentResponse struct {
	// in:body
	Body api.PagesDeploymentResponse `json:"body"`
}

// PagesDeploymentStatus
// swagger:response PagesDeploymentStatus
type swaggerResponsePagesDeploymentStatus struct {
	// in:body
	Body api.PagesDeploymentStatus `json:"body"`
}
