// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	api "code.gitea.io/gitea/modules/structs"
)

func issueTestPayload() *api.IssuePayload {
	return &api.IssuePayload{
		Index: 2,
		Sender: &api.User{
			UserName: "user1",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Issue: &api.Issue{
			ID:    2,
			Index: 2,
			URL:   "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			Title: "crash",
		},
	}
}

func issueCommentTestPayload() *api.IssueCommentPayload {
	return &api.IssueCommentPayload{
		Action: api.HookIssueCommentCreated,
		Sender: &api.User{
			UserName: "user1",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Comment: &api.Comment{
			HTMLURL:  "http://localhost:3000/test/repo/issues/2#issuecomment-4",
			IssueURL: "http://localhost:3000/test/repo/issues/2",
			Body:     "more info needed",
		},
		Issue: &api.Issue{
			ID:    2,
			Index: 2,
			URL:   "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			Title: "crash",
			Body:  "this happened",
		},
	}
}

func pullRequestCommentTestPayload() *api.IssueCommentPayload {
	return &api.IssueCommentPayload{
		Action: api.HookIssueCommentCreated,
		Sender: &api.User{
			UserName: "user1",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Comment: &api.Comment{
			HTMLURL: "http://localhost:3000/test/repo/pulls/2#issuecomment-4",
			PRURL:   "http://localhost:3000/test/repo/pulls/2",
			Body:    "changes requested",
		},
		Issue: &api.Issue{
			ID:    2,
			Index: 2,
			URL:   "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			Title: "Fix bug",
			Body:  "fixes bug #2",
		},
		IsPull: true,
	}
}

func pullReleaseTestPayload() *api.ReleasePayload {
	return &api.ReleasePayload{
		Action: api.HookReleasePublished,
		Sender: &api.User{
			UserName: "user1",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Release: &api.Release{
			TagName: "v1.0",
			Target:  "master",
			Title:   "First stable release",
			URL:     "http://localhost:3000/api/v1/repos/test/repo/releases/2",
		},
	}
}

func pullRequestTestPayload() *api.PullRequestPayload {
	return &api.PullRequestPayload{
		Action: api.HookIssueOpened,
		Index:  2,
		Sender: &api.User{
			UserName: "user1",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		PullRequest: &api.PullRequest{
			ID:        2,
			Index:     2,
			URL:       "http://localhost:3000/test/repo/pulls/12",
			Title:     "Fix bug",
			Body:      "fixes bug #2",
			Mergeable: true,
		},
	}
}
