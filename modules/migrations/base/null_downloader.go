// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"context"
	"net/url"
)

// NullNotifier implements a blank downloader
type NullDownloader struct {
}

var (
	_ Downloader = &NullDownloader{}
)

func (n NullDownloader) SetContext(_ context.Context) {
	return
}

func (n NullDownloader) GetRepoInfo() (*Repository, error) {
	return nil, &ErrNotSupported{}
}

func (n NullDownloader) GetTopics() ([]string, error) {
	return nil, &ErrNotSupported{}
}

func (n NullDownloader) GetMilestones() ([]*Milestone, error) {
	return nil, &ErrNotSupported{}
}

func (n NullDownloader) GetReleases() ([]*Release, error) {
	return nil, &ErrNotSupported{}
}

func (n NullDownloader) GetLabels() ([]*Label, error) {
	return nil, &ErrNotSupported{}
}

func (n NullDownloader) GetIssues(page, perPage int) ([]*Issue, bool, error) {
	return nil, false, &ErrNotSupported{}
}

func (n NullDownloader) GetComments(issueNumber int64) ([]*Comment, error) {
	return nil, &ErrNotSupported{}
}

func (n NullDownloader) GetPullRequests(page, perPage int) ([]*PullRequest, bool, error) {
	return nil, false, &ErrNotSupported{}
}

func (n NullDownloader) GetReviews(pullRequestNumber int64) ([]*Review, error) {
	return nil, &ErrNotSupported{}
}

func (n NullDownloader) FormatGitURL() func(opts MigrateOptions, remoteAddr string) (string, error) {
	return func(opts MigrateOptions, remoteAddr string) (string, error) {
		if len(opts.AuthToken) > 0 || len(opts.AuthUsername) > 0 {
			u, err := url.Parse(remoteAddr)
			if err != nil {
				return "", err
			}
			u.User = url.UserPassword(opts.AuthUsername, opts.AuthPassword)
			if len(opts.AuthToken) > 0 {
				u.User = url.UserPassword("oauth2", opts.AuthToken)
			}
			return u.String(), nil
		}
		return remoteAddr, nil
	}
}
