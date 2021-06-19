// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	api "code.gitea.io/gitea/modules/structs"
)

func createTestPayload() *api.CreatePayload {
	return &api.CreatePayload{
		Sha:     "2020558fe2e34debb818a514715839cabd25e777",
		Ref:     "refs/heads/test",
		RefType: "branch",
		Repo: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
	}
}

func deleteTestPayload() *api.DeletePayload {
	return &api.DeletePayload{
		Ref:     "refs/heads/test",
		RefType: "branch",
		Repo: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
	}
}

func forkTestPayload() *api.ForkPayload {
	return &api.ForkPayload{
		Forkee: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo2",
			Name:     "repo2",
			FullName: "test/repo2",
		},
		Repo: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
	}
}

func pushTestPayload() *api.PushPayload {
	commit := &api.PayloadCommit{
		ID:      "2020558fe2e34debb818a514715839cabd25e778",
		Message: "commit message",
		URL:     "http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778",
		Author: &api.PayloadUser{
			Name:     "user1",
			Email:    "user1@localhost",
			UserName: "user1",
		},
		Committer: &api.PayloadUser{
			Name:     "user1",
			Email:    "user1@localhost",
			UserName: "user1",
		},
	}

	return &api.PushPayload{
		Ref:        "refs/heads/test",
		Before:     "2020558fe2e34debb818a514715839cabd25e777",
		After:      "2020558fe2e34debb818a514715839cabd25e778",
		CompareURL: "",
		HeadCommit: commit,
		Commits:    []*api.PayloadCommit{commit, commit},
		Repo: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Pusher: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
	}
}

func issueTestPayload() *api.IssuePayload {
	return &api.IssuePayload{
		Index: 2,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Issue: &api.Issue{
			ID:      2,
			Index:   2,
			URL:     "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			HTMLURL: "http://localhost:3000/test/repo/issues/2",
			Title:   "crash",
			Body:    "issue body",
		},
	}
}

func issueCommentTestPayload() *api.IssueCommentPayload {
	return &api.IssueCommentPayload{
		Action: api.HookIssueCommentCreated,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
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
			ID:      2,
			Index:   2,
			URL:     "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			HTMLURL: "http://localhost:3000/test/repo/issues/2",
			Title:   "crash",
			Body:    "this happened",
		},
	}
}

func pullRequestCommentTestPayload() *api.IssueCommentPayload {
	return &api.IssueCommentPayload{
		Action: api.HookIssueCommentCreated,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
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
			ID:      2,
			Index:   2,
			URL:     "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			HTMLURL: "http://localhost:3000/test/repo/issues/2",
			Title:   "Fix bug",
			Body:    "fixes bug #2",
		},
		IsPull: true,
	}
}

func pullReleaseTestPayload() *api.ReleasePayload {
	return &api.ReleasePayload{
		Action: api.HookReleasePublished,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
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
			Note:    "Note of first stable release",
			URL:     "http://localhost:3000/api/v1/repos/test/repo/releases/2",
		},
	}
}

func pullRequestTestPayload() *api.PullRequestPayload {
	return &api.PullRequestPayload{
		Action: api.HookIssueOpened,
		Index:  12,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		PullRequest: &api.PullRequest{
			ID:        12,
			Index:     12,
			URL:       "http://localhost:3000/test/repo/pulls/12",
			HTMLURL:   "http://localhost:3000/test/repo/pulls/12",
			Title:     "Fix bug",
			Body:      "fixes bug #2",
			Mergeable: true,
		},
		Review: &api.ReviewPayload{
			Content: "good job",
		},
	}
}

func repositoryTestPayload() *api.RepositoryPayload {
	return &api.RepositoryPayload{
		Action: api.HookRepoCreated,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
	}
}
