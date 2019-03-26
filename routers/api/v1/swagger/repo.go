// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/sdk/gitea"
)

// Repository
// swagger:response Repository
type swaggerResponseRepository struct {
	// in:body
	Body api.Repository `json:"body"`
}

// RepositoryList
// swagger:response RepositoryList
type swaggerResponseRepositoryList struct {
	// in:body
	Body []api.Repository `json:"body"`
}

// Branch
// swagger:response Branch
type swaggerResponseBranch struct {
	// in:body
	Body api.Branch `json:"body"`
}

// BranchList
// swagger:response BranchList
type swaggerResponseBranchList struct {
	// in:body
	Body []api.Branch `json:"body"`
}

// TagList
// swagger:response TagList
type swaggerReponseTagList struct {
	// in:body
	Body []api.Tag `json:"body"`
}

// Reference
// swagger:response Reference
type swaggerResponseReference struct {
	// in:body
	Body api.Reference `json:"body"`
}

// ReferenceList
// swagger:response ReferenceList
type swaggerResponseReferenceList struct {
	// in:body
	Body []api.Reference `json:"body"`
}

// Hook
// swagger:response Hook
type swaggerResponseHook struct {
	// in:body
	Body api.Hook `json:"body"`
}

// HookList
// swagger:response HookList
type swaggerResponseHookList struct {
	// in:body
	Body []api.Hook `json:"body"`
}

// Release
// swagger:response Release
type swaggerResponseRelease struct {
	// in:body
	Body api.Release `json:"body"`
}

// ReleaseList
// swagger:response ReleaseList
type swaggerResponseReleaseList struct {
	// in:body
	Body []api.Release `json:"body"`
}

// PullRequest
// swagger:response PullRequest
type swaggerResponsePullRequest struct {
	// in:body
	Body api.PullRequest `json:"body"`
}

// PullRequestList
// swagger:response PullRequestList
type swaggerResponsePullRequestList struct {
	// in:body
	Body []api.PullRequest `json:"body"`
}

// Status
// swagger:response Status
type swaggerResponseStatus struct {
	// in:body
	Body api.Status `json:"body"`
}

// StatusList
// swagger:response StatusList
type swaggerResponseStatusList struct {
	// in:body
	Body []api.Status `json:"body"`
}

// WatchInfo
// swagger:response WatchInfo
type swaggerResponseWatchInfo struct {
	// in:body
	Body api.WatchInfo `json:"body"`
}

// SearchResults
// swagger:response SearchResults
type swaggerResponseSearchResults struct {
	// in:body
	Body api.SearchResults `json:"body"`
}

// AttachmentList
// swagger:response AttachmentList
type swaggerResponseAttachmentList struct {
	//in: body
	Body []api.Attachment `json:"body"`
}

// Attachment
// swagger:response Attachment
type swaggerResponseAttachment struct {
	//in: body
	Body api.Attachment `json:"body"`
}

// GitTreeResponse
// swagger:response GitTreeResponse
type swaggerGitTreeResponse struct {
	//in: body
	Body api.GitTreeResponse `json:"body"`
}

// Commit
// swagger:response Commit
type swaggerCommit struct {
	//in: body
	Body api.Commit `json:"body"`
}
