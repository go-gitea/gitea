// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"

	base "gitea.dev/modules/migration"
)

type githubIssueRecoveryUploader struct {
	base.Uploader
	downloader *GithubDownloaderV3
	issues     map[int64]struct{}
}

func wrapGithubIssueRecoveryUploader(uploader base.Uploader, downloader base.Downloader) base.Uploader {
	if retry, ok := downloader.(*base.RetryDownloader); ok {
		downloader = retry.Downloader
	}
	github, ok := downloader.(*GithubDownloaderV3)
	if !ok {
		return uploader
	}
	return &githubIssueRecoveryUploader{
		Uploader:   uploader,
		downloader: github,
		issues:     make(map[int64]struct{}),
	}
}

func (g *githubIssueRecoveryUploader) CreateIssues(ctx context.Context, issues ...*base.Issue) error {
	if err := g.Uploader.CreateIssues(ctx, issues...); err != nil {
		return err
	}
	for _, issue := range issues {
		g.issues[issue.Number] = struct{}{}
	}
	return nil
}

func (g *githubIssueRecoveryUploader) CreatePullRequests(ctx context.Context, prs ...*base.PullRequest) error {
	if err := g.Uploader.CreatePullRequests(ctx, prs...); err != nil {
		return err
	}
	for _, pr := range prs {
		g.issues[pr.Number] = struct{}{}
	}
	return nil
}

func (g *githubIssueRecoveryUploader) CreateComments(ctx context.Context, comments ...*base.Comment) error {
	for _, comment := range comments {
		if _, ok := g.issues[comment.IssueIndex]; ok {
			continue
		}

		issue, err := g.downloader.GetIssue(ctx, comment.IssueIndex)
		if err != nil {
			return fmt.Errorf("recover missing IssueIndex %d: %w", comment.IssueIndex, err)
		}
		if err := g.Uploader.CreateIssues(ctx, issue); err != nil {
			return err
		}
		g.issues[issue.Number] = struct{}{}
	}
	return g.Uploader.CreateComments(ctx, comments...)
}
