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

// PagesVersion describes one retained Pages artifact.
type PagesVersion struct {
	ID                int64  `json:"id"`
	TarGzSHA256       string `json:"tar_gz_sha256"`
	OriginType        string `json:"origin_type"`
	OriginActionRunID int64  `json:"origin_action_run_id"`
	Comment           string `json:"comment"`
	KeepForever       bool   `json:"keep_forever"`
	CreatedUnix       int64  `json:"created_unix"`
	Current           bool   `json:"current"`
}

// EditPagesVersionOption changes mutable metadata for a retained Pages version.
type EditPagesVersionOption struct {
	Comment     *string `json:"comment"`
	KeepForever *bool   `json:"keep_forever"`
}

// PagesContentCommit describes the prepared Pages version and current state.
type PagesContentCommit struct {
	Version              *PagesVersion `json:"version"`
	PagesUpdatedUnixNano int64         `json:"pages_updated_unix_nano"`
}
