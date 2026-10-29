// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// PagesDeploymentOption is the request accepted by the Actions Pages
// compatibility endpoint. The values identify the current Actions run and
// are checked against the authenticated task before the artifact is read.
type PagesDeploymentOption struct {
	ArtifactID        int64  `json:"artifact_id" form:"artifact_id" binding:"Required"`
	PagesBuildVersion string `json:"pages_build_version" form:"pages_build_version" binding:"Required"`
	OIDCToken         string `json:"oidc_token" form:"oidc_token" binding:"Required"`
	Preview           bool   `json:"preview" form:"preview"`
}

// PagesDeploymentResponse is returned after the artifact has been checked and
// made the repository's current Pages content.
type PagesDeploymentResponse struct {
	ID        string `json:"id"`
	StatusURL string `json:"status_url"`
	PageURL   string `json:"page_url,omitempty"`
}

// PagesDeploymentStatus describes whether every registered Pages instance has
// published the submitted digest at its default address.
type PagesDeploymentStatus struct {
	Status string `json:"status"`
}

// PagesContentCommit describes the current prepared Pages artifact.
type PagesContentCommit struct {
	TarGzSHA256          string `json:"tar_gz_sha256"`
	PagesUpdatedUnixNano int64  `json:"pages_updated_unix_nano"`
}
