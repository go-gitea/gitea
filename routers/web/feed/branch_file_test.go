// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed_test

import (
	"fmt"
	"html"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const feedCommitMessage = "fix <script>alert(1)</script>"

func setupFeedTestRepo(t *testing.T) (*git.Repository, *git.Commit) {
	repoPath := t.TempDir()
	stdin := fmt.Sprintf(`commit refs/heads/master
author Test <test@example.com> 1700000000 +0000
committer Test <test@example.com> 1700000000 +0000
data <<EOT
%s
EOT
M 100644 inline test.txt
data <<EOT
content
EOT
`, feedCommitMessage)

	require.NoError(t, gitcmd.NewCommand("init", "--bare", ".").WithDir(repoPath).RunWithStderr(t.Context()))
	require.NoError(t, gitcmd.NewCommand("fast-import").WithDir(repoPath).WithStdinBytes([]byte(stdin)).RunWithStderr(t.Context()))

	gitRepo, err := git.OpenRepository(t.Context(), repoPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = gitRepo.Close()
	})

	commit, err := gitRepo.GetBranchCommit("master")
	require.NoError(t, err)

	return gitRepo, commit
}

func TestShowBranchFeedEscapesCommitMessage(t *testing.T) {
	gitRepo, commit := setupFeedTestRepo(t)
	ctx, resp := contexttest.MockContext(t, "/")
	ctx.Repo.Commit = commit
	ctx.Repo.GitRepo = gitRepo
	ctx.Repo.RefFullName = git.RefNameFromBranch("master")
	ctx.Repo.BranchName = "master"

	repo := &repo_model.Repository{OwnerName: "test", Name: "repo"}
	feed.ShowBranchFeed(ctx, repo, "rss")

	body := resp.Body.String()
	expected := html.EscapeString(feedCommitMessage)
	assert.Contains(t, body, expected)
	assert.NotContains(t, body, "<script>")
}

func TestShowFileFeedEscapesCommitMessage(t *testing.T) {
	gitRepo, commit := setupFeedTestRepo(t)
	ctx, resp := contexttest.MockContext(t, "/")
	ctx.Repo.Commit = commit
	ctx.Repo.GitRepo = gitRepo
	ctx.Repo.RefFullName = git.RefNameFromBranch("master")
	ctx.Repo.BranchName = "master"
	ctx.Repo.TreePath = "test.txt"

	repo := &repo_model.Repository{OwnerName: "test", Name: "repo"}
	feed.ShowFileFeed(ctx, repo, "rss")

	body := resp.Body.String()
	expected := html.EscapeString(feedCommitMessage)
	assert.Contains(t, body, expected)
	assert.NotContains(t, body, "<script>")
}
