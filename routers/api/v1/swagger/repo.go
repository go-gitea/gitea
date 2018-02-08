// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/sdk/gitea"
)

// swagger:response Repository
type swaggerResponseRepository struct {
	// in:body
	Body api.Repository `json:"body"`
}

// swagger:response RepositoryList
type swaggerResponseRepositoryList struct {
	// in:body
	Body []api.Repository `json:"body"`
}

// swagger:response Branch
type swaggerResponseBranch struct {
	// in:body
	Body api.Branch `json:"body"`
}

// swagger:response BranchList
type swaggerResponseBranchList struct {
	// in:body
	Body []api.Branch `json:"body"`
}

// swagger:response Hook
type swaggerResponseHook struct {
	// in:body
	Body []api.Branch `json:"body"`
}

// swagger:response HookList
type swaggerResponseHookList struct {
	// in:body
	Body []api.Branch `json:"body"`
}

// swagger:response Release
type swaggerResponseRelease struct {
	// in:body
	Body api.Release `json:"body"`
}

// swagger:response ReleaseList
type swaggerResponseReleaseList struct {
	// in:body
	Body []api.Release `json:"body"`
}

// swagger:response PullRequest
type swaggerResponsePullRequest struct {
	// in:body
	Body api.PullRequest `json:"body"`
}

// swagger:response PullRequestList
type swaggerResponsePullRequestList struct {
	// in:body
	Body []api.PullRequest `json:"body"`
}

// swagger:response Status
type swaggerResponseStatus struct {
	// in:body
	Body api.Status `json:"body"`
}

// swagger:response StatusList
type swaggerResponseStatusList struct {
	// in:body
	Body []api.Status `json:"body"`
}

// swagger:response WatchInfo
type swaggerResponseWatchInfo struct {
	// in:body
	Body api.WatchInfo `json:"body"`
}

// swagger:response SearchResults
type swaggerResponseSearchResults struct {
	Body api.SearchResults `json:"body"`
}

// swagger:response AttachmentList
type swaggerResponseAttachmentList struct {
	//in: body
	Body []api.Attachment `json:"body"`
}

// swagger:response Attachment
type swaggerResponseAttachment struct {
	//in: body
	Body api.Attachment `json:"body"`
}
