// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/url"

	"gitea.dev/modules/git"
	"gitea.dev/modules/glob"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/gitdiff"
)

// diffViewerConfig is the page data the client-side diff viewer is driven by
type diffViewerConfig struct {
	StreamURL        string `json:"streamUrl"`
	CommentsURL      string `json:"commentsUrl,omitempty"`
	IssueLink        string `json:"issueLink,omitempty"`
	RepoLink         string `json:"repoLink,omitempty"` // to browse the commits, empty for the wiki which can't
	BeforeCommitID   string `json:"beforeCommitId,omitempty"`
	AfterCommitID    string `json:"afterCommitId"`
	HeadCommitID     string `json:"headCommitId,omitempty"` // new comments are attached to it, even when older commits are shown
	EditFileLink     string `json:"editFileLink,omitempty"` // the file path is appended
	EditReturnURI    string `json:"editReturnUri,omitempty"`
	IsSplit          bool   `json:"isSplit"`
	CanReviewFiles   bool   `json:"canReviewFiles"`
	MaxFileSize      int64  `json:"maxFileSize"`
	CsvMaxFileSize   int64  `json:"csvMaxFileSize"`
	DisableHighlight bool   `json:"disableHighlight"`
	DetectAmbiguous  bool   `json:"detectAmbiguous"`
}

type diffViewerOptions struct {
	streamPath     string
	streamQuery    url.Values
	gitRepoLink    string // the repo the commits are browsed in, the head repo for compare, empty for the wiki which can't
	beforeCommitID string // empty for a root commit
	afterCommitID  string
}

func prepareDiffViewer(ctx *context.Context, opts diffViewerOptions) *diffViewerConfig {
	if opts.streamQuery == nil {
		opts.streamQuery = url.Values{}
	}
	// the stream uses the same whitespace option as the page, which isn't persisted for anonymous users
	opts.streamQuery.Set("whitespace", GetWhitespaceBehavior(ctx))
	config := &diffViewerConfig{
		StreamURL:        opts.streamPath + "?" + opts.streamQuery.Encode(),
		RepoLink:         opts.gitRepoLink,
		BeforeCommitID:   opts.beforeCommitID,
		AfterCommitID:    opts.afterCommitID,
		IsSplit:          GetDiffViewStyle(ctx) == gitdiff.DiffStyleSplit,
		MaxFileSize:      setting.Git.MaxDiffViewFileSize,
		CsvMaxFileSize:   setting.UI.CSV.MaxFileSize,
		DisableHighlight: setting.Git.DisableDiffHighlight,
		DetectAmbiguous:  setting.UI.AmbiguousUnicodeDetection,
	}
	ctx.PageData["diffViewer"] = config
	return config
}

// serveDiffStream writes the files of a diff as newline-delimited JSON, the last line tells whether the diff was truncated.
// The options name the commits and the repo, the request selects the files and whether a file is loaded beyond the size limit.
func serveDiffStream(ctx *context.Context, gitRepo *git.Repository, opts gitdiff.DiffStreamOptions, decorate func(*gitdiff.DiffStreamFile)) {
	if opts.AfterCommitID == "" {
		ctx.NotFound(nil)
		return
	}
	opts.WhitespaceBehavior = gitdiff.GetWhitespaceFlag(ctx.FormString("whitespace")) // the page's option, see prepareDiffViewer
	opts.Files = ctx.FormStrings("files")
	opts.MaxSize = setting.Git.MaxDiffViewSize
	opts.MaxFileSize = util.Iif(ctx.FormBool("full") && len(opts.Files) > 0, -1, setting.Git.MaxDiffViewFileSize)
	ec, _, _ := ctx.Repo.GetEditorconfig(ctx)
	writeLine := func(v any) error {
		line, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if !ctx.Written() {
			ctx.Resp.Header().Set("Content-Type", "application/x-ndjson")
			ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
		}
		if _, err = ctx.Resp.Write(append(line, '\n')); err != nil {
			return err
		}
		ctx.Resp.Flush()
		return nil
	}
	truncated, err := gitdiff.StreamDiff(ctx, gitRepo, &opts, func(file *gitdiff.DiffStreamFile) error {
		file.TabSize = templates.TabWidth(ec, file.Name)
		if decorate != nil {
			decorate(file)
		}
		return writeLine(file)
	})
	if err != nil && !ctx.Written() { // a failure before the first file still gets a status
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("StreamDiff", err)
		}
		return
	}
	if err != nil && ctx.Err() == nil { // not when the client has left
		log.Error("serveDiffStream: %v", err)
	}
	_ = writeLine(map[string]any{"end": true, "truncated": truncated, "failed": err != nil})
}

// DiffStream streams the diff between two commits of the repository
func DiffStream(ctx *context.Context) {
	serveDiffStream(ctx, ctx.Repo.GitRepo, gitdiff.DiffStreamOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{BeforeCommitID: ctx.FormString("before"), AfterCommitID: ctx.FormString("after")},
		RepoID:            ctx.Repo.Repository.ID,
		RepoLink:          ctx.Repo.RepoLink,
	}, nil)
}

// WikiDiffStream streams the diff between two commits of the wiki
func WikiDiffStream(ctx *context.Context) {
	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository.WikiStorageRepo())
	if err != nil {
		ctx.ServerError("RepositoryFromRequestContextOrOpen", err)
		return
	}
	serveDiffStream(ctx, gitRepo, gitdiff.DiffStreamOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{BeforeCommitID: ctx.FormString("before"), AfterCommitID: ctx.FormString("after")},
		RepoLink:          ctx.Repo.RepoLink + "/wiki",
	}, nil)
}

// PullFilesStream streams the diff of a pull request, "from" and "to" select commits like the "files" page URLs
func PullFilesStream(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	filesRange := preparePullFilesRange(ctx, issue, ctx.FormString("from"), ctx.FormString("to"))
	if ctx.Written() {
		return
	}
	rule := filesRange.prViewInfo.ProtectedBranchRule
	var patterns []glob.Glob
	if rule != nil {
		patterns = rule.GetProtectedFilePatterns()
	}
	serveDiffStream(ctx, ctx.Repo.GitRepo, gitdiff.DiffStreamOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{BeforeCommitID: filesRange.beforeCommit.ID.String(), AfterCommitID: filesRange.afterCommit.ID.String()},
		RepoID:            ctx.Repo.Repository.ID,
		RepoLink:          ctx.Repo.RepoLink,
	}, func(file *gitdiff.DiffStreamFile) {
		file.IsProtected = len(patterns) != 0 && rule.IsProtectedFile(patterns, file.Name)
	})
}
