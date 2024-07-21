// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
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

// BranchProtection
// swagger:response BranchProtection
type swaggerResponseBranchProtection struct {
	// in:body
	Body api.BranchProtection `json:"body"`
}

// BranchProtectionList
// swagger:response BranchProtectionList
type swaggerResponseBranchProtectionList struct {
	// in:body
	Body []api.BranchProtection `json:"body"`
}

// TagList
// swagger:response TagList
type swaggerResponseTagList struct {
	// in:body
	Body []api.Tag `json:"body"`
}

// Tag
// swagger:response Tag
type swaggerResponseTag struct {
	// in:body
	Body api.Tag `json:"body"`
}

// AnnotatedTag
// swagger:response AnnotatedTag
type swaggerResponseAnnotatedTag struct {
	// in:body
	Body api.AnnotatedTag `json:"body"`
}

// TagProtectionList
// swagger:response TagProtectionList
type swaggerResponseTagProtectionList struct {
	// in:body
	Body []api.TagProtection `json:"body"`
}

// TagProtection
// swagger:response TagProtection
type swaggerResponseTagProtection struct {
	// in:body
	Body api.TagProtection `json:"body"`
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

// GitHook
// swagger:response GitHook
type swaggerResponseGitHook struct {
	// in:body
	Body api.GitHook `json:"body"`
}

// GitHookList
// swagger:response GitHookList
type swaggerResponseGitHookList struct {
	// in:body
	Body []api.GitHook `json:"body"`
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

// PullReview
// swagger:response PullReview
type swaggerResponsePullReview struct {
	// in:body
	Body api.PullReview `json:"body"`
}

// PullReviewList
// swagger:response PullReviewList
type swaggerResponsePullReviewList struct {
	// in:body
	Body []api.PullReview `json:"body"`
}

// PullComment
// swagger:response PullReviewComment
type swaggerPullReviewComment struct {
	// in:body
	Body api.PullReviewComment `json:"body"`
}

// PullCommentList
// swagger:response PullReviewCommentList
type swaggerResponsePullReviewCommentList struct {
	// in:body
	Body []api.PullReviewComment `json:"body"`
}

// CommitStatus
// swagger:response CommitStatus
type swaggerResponseStatus struct {
	// in:body
	Body api.CommitStatus `json:"body"`
}

// CommitStatusList
// swagger:response CommitStatusList
type swaggerResponseCommitStatusList struct {
	// in:body
	Body []api.CommitStatus `json:"body"`
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
	// in: body
	Body []api.Attachment `json:"body"`
}

// Attachment
// swagger:response Attachment
type swaggerResponseAttachment struct {
	// in: body
	Body api.Attachment `json:"body"`
}

// GitTreeResponse
// swagger:response GitTreeResponse
type swaggerGitTreeResponse struct {
	// in: body
	Body api.GitTreeResponse `json:"body"`
}

// GitBlobResponse
// swagger:response GitBlobResponse
type swaggerGitBlobResponse struct {
	// in: body
	Body api.GitBlobResponse `json:"body"`
}

// Commit
// swagger:response Commit
type swaggerCommit struct {
	// in: body
	Body api.Commit `json:"body"`
}

// CommitList
// swagger:response CommitList
type swaggerCommitList struct {
	// The current page
	Page int `json:"X-Page"`

	// Commits per page
	PerPage int `json:"X-PerPage"`

	// Total commit count
	Total int `json:"X-Total"`

	// Total number of pages
	PageCount int `json:"X-PageCount"`

	// True if there is another page
	HasMore bool `json:"X-HasMore"`

	// in: body
	Body []api.Commit `json:"body"`
}

// ChangedFileList
// swagger:response ChangedFileList
type swaggerChangedFileList struct {
	// The current page
	Page int `json:"X-Page"`

	// Commits per page
	PerPage int `json:"X-PerPage"`

	// Total commit count
	Total int `json:"X-Total"`

	// Total number of pages
	PageCount int `json:"X-PageCount"`

	// True if there is another page
	HasMore bool `json:"X-HasMore"`

	// in: body
	Body []api.ChangedFile `json:"body"`
}

// Note
// swagger:response Note
type swaggerNote struct {
	// in: body
	Body api.Note `json:"body"`
}

// EmptyRepository
// swagger:response EmptyRepository
type swaggerEmptyRepository struct {
	// in: body
	Body api.APIError `json:"body"`
}

// FileResponse
// swagger:response FileResponse
type swaggerFileResponse struct {
	// in: body
	Body api.FileResponse `json:"body"`
}

// FilesResponse
// swagger:response FilesResponse
type swaggerFilesResponse struct {
	// in: body
	Body api.FilesResponse `json:"body"`
}

// ContentsResponse
// swagger:response ContentsResponse
type swaggerContentsResponse struct {
	// in: body
	Body api.ContentsResponse `json:"body"`
}

// ContentsListResponse
// swagger:response ContentsListResponse
type swaggerContentsListResponse struct {
	// in:body
	Body []api.ContentsResponse `json:"body"`
}

// FileDeleteResponse
// swagger:response FileDeleteResponse
type swaggerFileDeleteResponse struct {
	// in: body
	Body api.FileDeleteResponse `json:"body"`
}

// TopicListResponse
// swagger:response TopicListResponse
type swaggerTopicListResponse struct {
	// in: body
	Body []api.TopicResponse `json:"body"`
}

// TopicNames
// swagger:response TopicNames
type swaggerTopicNames struct {
	// in: body
	Body api.TopicName `json:"body"`
}

// LanguageStatistics
// swagger:response LanguageStatistics
type swaggerLanguageStatistics struct {
	// in: body
	Body map[string]int64 `json:"body"`
}

// CombinedStatus
// swagger:response CombinedStatus
type swaggerCombinedStatus struct {
	// in: body
	Body api.CombinedStatus `json:"body"`
}

// WikiPageList
// swagger:response WikiPageList
type swaggerWikiPageList struct {
	// in:body
	Body []api.WikiPageMetaData `json:"body"`
}

// WikiPage
// swagger:response WikiPage
type swaggerWikiPage struct {
	// in:body
	Body api.WikiPage `json:"body"`
}

// WikiCommitList
// swagger:response WikiCommitList
type swaggerWikiCommitList struct {
	// in:body
	Body api.WikiCommitList `json:"body"`
}

// PushMirror
// swagger:response PushMirror
type swaggerPushMirror struct {
	// in:body
	Body api.PushMirror `json:"body"`
}

// PushMirrorList
// swagger:response PushMirrorList
type swaggerPushMirrorList struct {
	// in:body
	Body []api.PushMirror `json:"body"`
}

// RepoCollaboratorPermission
// swagger:response RepoCollaboratorPermission
type swaggerRepoCollaboratorPermission struct {
	// in:body
	Body api.RepoCollaboratorPermission `json:"body"`
}

// RepoIssueConfig
// swagger:response RepoIssueConfig
type swaggerRepoIssueConfig struct {
	// in:body
	Body api.IssueConfig `json:"body"`
}

// RepoIssueConfigValidation
// swagger:response RepoIssueConfigValidation
type swaggerRepoIssueConfigValidation struct {
	// in:body
	Body api.IssueConfigValidation `json:"body"`
}

// RepoNewIssuePinsAllowed
// swagger:response RepoNewIssuePinsAllowed
type swaggerRepoNewIssuePinsAllowed struct {
	// in:body
	Body api.NewIssuePinsAllowed `json:"body"`
}

// TasksList
// swagger:response TasksList
type swaggerRepoTasksList struct {
	// in:body
	Body api.ActionTaskResponse `json:"body"`
}

// swagger:response Compare
type swaggerCompare struct {
	// in:body
	Body api.Compare `json:"body"`
}
