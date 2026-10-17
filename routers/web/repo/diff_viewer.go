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

// serveDiffStream writes the files of a diff as newline-delimited JSON, the last line tells whether the diff was truncated
func serveDiffStream(ctx *context.Context, gitRepo *git.Repository, repoID int64, repoLink, beforeCommitID, afterCommitID string, decorate func(*gitdiff.DiffStreamFile)) {
	if afterCommitID == "" {
		ctx.NotFound(nil)
		return
	}
	for _, commitID := range []string{beforeCommitID, afterCommitID} {
		if commitID == "" {
			continue
		}
		if _, err := gitRepo.GetCommit(ctx, commitID); err != nil {
			if git.IsErrNotExist(err) {
				ctx.NotFound(err)
			} else {
				ctx.ServerError("GetCommit", err)
			}
			return
		}
	}

	opts := &gitdiff.DiffStreamOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{
			BeforeCommitID:     beforeCommitID,
			AfterCommitID:      afterCommitID,
			WhitespaceBehavior: gitdiff.GetWhitespaceFlag(GetWhitespaceBehavior(ctx)),
		},
		Files:       ctx.FormStrings("files"),
		MaxSize:     setting.Git.MaxDiffViewSize,
		MaxFileSize: util.Iif(ctx.FormBool("full"), -1, setting.Git.MaxDiffViewFileSize),
		RepoID:      repoID,
		RepoLink:    repoLink,
	}
	ec, _, _ := ctx.Repo.GetEditorconfig(ctx)
	ctx.Resp.Header().Set("Content-Type", "application/x-ndjson")
	ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
	writeLine := func(v any) error {
		line, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = ctx.Resp.Write(append(line, '\n'))
		return err
	}
	truncated, err := gitdiff.StreamDiff(ctx, gitRepo, opts, func(file *gitdiff.DiffStreamFile) error {
		file.TabSize = templates.TabWidth(ec, file.Name)
		if decorate != nil {
			decorate(file)
		}
		if err := writeLine(file); err != nil {
			return err
		}
		ctx.Resp.Flush()
		return nil
	})
	if err != nil && ctx.Err() == nil { // not when the client has left
		log.Error("serveDiffStream: %v", err)
	}
	_ = writeLine(map[string]any{"end": true, "truncated": truncated, "failed": err != nil})
}

// DiffStream streams the diff between two commits of the repository
func DiffStream(ctx *context.Context) {
	serveDiffStream(ctx, ctx.Repo.GitRepo, ctx.Repo.Repository.ID, ctx.Repo.RepoLink, ctx.FormString("before"), ctx.FormString("after"), nil)
}

// WikiDiffStream streams the diff between two commits of the wiki
func WikiDiffStream(ctx *context.Context) {
	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository.WikiStorageRepo())
	if err != nil {
		ctx.ServerError("RepositoryFromRequestContextOrOpen", err)
		return
	}
	serveDiffStream(ctx, gitRepo, 0, ctx.Repo.RepoLink+"/wiki", ctx.FormString("before"), ctx.FormString("after"), nil)
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
	serveDiffStream(ctx, ctx.Repo.GitRepo, ctx.Repo.Repository.ID, ctx.Repo.RepoLink, filesRange.beforeCommit.ID.String(), filesRange.afterCommit.ID.String(), func(file *gitdiff.DiffStreamFile) {
		file.IsProtected = len(patterns) != 0 && rule.IsProtectedFile(patterns, file.Name)
	})
}
