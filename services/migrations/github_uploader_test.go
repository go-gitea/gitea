// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/services/pull"

	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGithubTestUploader(t *testing.T) *GiteaLocalUploader {
	t.Helper()
	unittest.PrepareTestEnv(t)
	local := NewGiteaLocalUploader(t.Context(), unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}), "user2", "repo1")
	local.repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	local.gitServiceType = structs.GithubService
	return local
}

func openGithubTestRepository(t *testing.T, local *GiteaLocalUploader) {
	t.Helper()
	var err error
	local.gitRepo, err = git.OpenRepository(t.Context(), local.repo)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, local.gitRepo.Close()) })
}

func TestGithubPatchTooLarge(t *testing.T) {
	defer test.MockVariableValue(&setting.RepoRootPath, t.TempDir())()
	repo := &repo_model.Repository{OwnerName: "owner", Name: "repo"}
	require.NoError(t, git.InitRepository(t.Context(), repo, "sha1", "base"))
	var history strings.Builder
	history.WriteString("commit refs/heads/base\nmark :1\ncommitter Test <test@example.com> 1 +0000\ndata 4\nbase\nM 100644 inline conflict.txt\ndata 5\nbase\n\nM 100644 inline binary.dat\ndata 4\n\x00old\n\n")
	history.WriteString("commit refs/heads/side\nmark :2\ncommitter Test <test@example.com> 2 +0000\ndata 4\nside\nfrom :1\nM 100644 inline conflict.txt\ndata 5\nside\n\n")
	history.WriteString("commit refs/heads/feature\nmark :3\ncommitter Test <test@example.com> 3 +0000\ndata 4\nhead\nfrom :1\nM 100644 inline conflict.txt\ndata 5\nhead\n\nM 100644 inline binary.dat\ndata 4\n\x00new\n")
	for i := range 301 {
		fmt.Fprintf(&history, "M 100644 inline file-%03d.txt\ndata 4\nnew\n\n", i)
	}
	history.WriteString("\ncommit refs/heads/feature\ncommitter Test <test@example.com> 4 +0000\ndata 5\nmerge\nfrom :3\nmerge :2\nM 100644 inline conflict.txt\ndata 9\nresolved\n\n")
	history.WriteString("commit refs/heads/base\ncommitter Test <test@example.com> 5 +0000\ndata 4\nbase\nfrom :1\nM 100644 inline base-only.txt\ndata 4\nbase\n\n")
	require.NoError(t, gitcmd.NewCommand("fast-import").WithRepo(repo).WithStdinBytes([]byte(history.String())).Run(t.Context()))
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	baseSHA, err := gitRepo.GetRefCommitID(t.Context(), "refs/heads/base")
	require.NoError(t, err)
	headSHA, err := gitRepo.GetRefCommitID(t.Context(), "refs/heads/feature")
	require.NoError(t, err)
	mergeBase, err := git.MergeBase(t.Context(), repo, baseSHA, headSHA)
	require.NoError(t, err)
	head, err := gitRepo.GetCommit(t.Context(), headSHA)
	require.NoError(t, err)
	assert.Equal(t, 2, head.ParentCount())

	for i, example := range []struct {
		name    string
		status  int
		code    string
		noGit   bool
		head    string
		wantErr string
	}{
		{name: "download", status: http.StatusOK, head: headSHA},
		{name: "too_large", status: http.StatusNotAcceptable, code: "too_large", head: headSHA},
		{name: "other_406", status: http.StatusNotAcceptable, code: "invalid", head: headSHA, wantErr: "406"},
		{name: "forbidden", status: http.StatusForbidden, code: "too_large", head: headSHA, wantErr: "403"},
		{name: "server_error", status: http.StatusInternalServerError, code: "too_large", head: headSHA, wantErr: "500"},
		{name: "without_git", status: http.StatusNotAcceptable, code: "too_large", noGit: true, head: headSHA, wantErr: "406"},
		{name: "missing_commit", status: http.StatusNotAcceptable, code: "too_large", head: strings.Repeat("0", 40), wantErr: "generate PR"},
		{name: "missing_sha", status: http.StatusNotAcceptable, code: "too_large", wantErr: "has no head or base commit"},
	} {
		t.Run(example.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/vnd.github.v3.patch", r.Header.Get("Accept"))
				w.Header().Set("X-RateLimit-Remaining", "100")
				w.WriteHeader(example.status)
				if example.status == http.StatusOK {
					_, _ = io.WriteString(w, "remote patch")
					return
				}
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"message": "patch unavailable", "errors": []github.Error{{Resource: "PullRequest", Field: "diff", Code: example.code}}}))
			}))
			defer server.Close()
			downloader, err := NewGithubDownloaderV3(t.Context(), server.URL, "", "", "", "o", "r")
			require.NoError(t, err)
			uploader := &githubUploader{GiteaLocalUploader: &GiteaLocalUploader{repo: repo, gitRepo: gitRepo}, downloader: downloader}
			if example.noGit {
				uploader.gitRepo = nil
			}
			pr := &base.PullRequest{Number: int64(881 + i), Base: base.PullRequestBranch{SHA: baseSHA}, Head: base.PullRequestBranch{SHA: example.head}}
			err = uploader.downloadPatch(t.Context(), pr)
			if example.wantErr != "" {
				require.ErrorContains(t, err, example.wantErr)
				return
			}
			require.NoError(t, err)
			patch, err := os.ReadFile(filepath.Join(gitrepo.RepoLocalPath(repo), fmt.Sprintf("pulls/%d.patch", pr.Number)))
			require.NoError(t, err)
			if example.status == http.StatusOK {
				assert.Equal(t, "remote patch", string(patch))
				return
			}
			assert.Equal(t, 303, strings.Count(string(patch), "diff --git "))
			assert.Contains(t, string(patch), "GIT binary patch")
			assert.Contains(t, string(patch), "+resolved")
			assert.NotContains(t, string(patch), "base-only.txt")
			env := append(gitcmd.CommonGitCmdEnvs(), "GIT_INDEX_FILE="+filepath.Join(t.TempDir(), "index"))
			require.NoError(t, gitcmd.NewCommand("read-tree").AddDynamicArguments(mergeBase).WithRepo(repo).WithEnv(env).Run(t.Context()))
			require.NoError(t, gitcmd.NewCommand("apply", "--cached").WithRepo(repo).WithEnv(env).WithStdinBytes(patch).Run(t.Context()))
			tree, _, err := gitcmd.NewCommand("write-tree").WithRepo(repo).WithEnv(env).RunStdString(t.Context())
			require.NoError(t, err)
			assert.Equal(t, head.TreeID.String(), strings.TrimSpace(tree))
		})
	}
}

func TestGithubReviewPreservation(t *testing.T) {
	local := newGithubTestUploader(t)
	require.NoError(t, local.CreateIssues(t.Context(), &base.Issue{Number: 4877, Title: "PR", Content: "See #1"}))
	uploader := &githubUploader{GiteaLocalUploader: local}
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, uploader.CreateComments(t.Context(),
		&base.Comment{IssueIndex: 4877, Index: 100, Content: "discussion", Created: created},
		&base.Comment{IssueIndex: 4877, CommentType: "change_title", Created: created, Meta: map[string]any{"OldTitle": "old", "NewTitle": "PR"}},
	))
	assert.Len(t, uploader.comments, 1)
	assert.Equal(t, "discussion", uploader.comments[100].Content)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: local.issues[4877].ID, Type: issues_model.CommentTypeChangeTitle, OldTitle: "old", NewTitle: "PR"})
	parent := &base.ReviewComment{ID: 101, Content: "parent #1", TreePath: "file.go", CommitID: "0123456789012345678901234567890123456789", Line: -4, DiffHunk: "@@ -4 +4 @@\n-old\n+new", CreatedAt: created, UpdatedAt: created, Reactions: []*base.Reaction{{UserID: 2, UserName: "user2", Content: "+1"}}}
	child := *parent
	parent.Resolved = true
	child.ID, child.InReplyTo, child.Content, child.Reactions = 102, 101, "reply", nil
	// Replies can arrive in an earlier batch than their parent.
	require.NoError(t, uploader.CreateReviews(t.Context(), &base.Review{ID: 2, IssueIndex: 4877, State: base.ReviewStateCommented, Comments: []*base.ReviewComment{&child}}))
	require.NoError(t, uploader.CreateReviews(t.Context(), &base.Review{ID: 1, IssueIndex: 4877, State: base.ReviewStateCommented, Dismissed: true, Comments: []*base.ReviewComment{parent}}))
	require.NoError(t, uploader.Finish(t.Context()))
	p := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: uploader.reviewComments[101].ID})
	c := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: uploader.reviewComments[102].ID})
	assert.Equal(t, local.doer.ID, p.ResolveDoerID)
	assert.Zero(t, c.ResolveDoerID)
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: p.ReviewID}).Dismissed)
	assert.Equal(t, int64(-4), p.Line)
	assert.Equal(t, parent.DiffHunk, p.Patch)
	assert.Equal(t, parent.CommitID, p.CommitSHA)
	assert.Equal(t, created.Unix(), int64(p.CreatedUnix))
	assert.Equal(t, created.Unix(), int64(c.UpdatedUnix))
	assert.Contains(t, c.Content, p.HTMLURL(t.Context()))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{CommentID: p.ID, Type: "+1"})
	target := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: local.repo.ID, Index: 1})
	source := local.issues[4877]
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: target.ID, RefIssueID: source.ID, Type: issues_model.CommentTypeIssueRef})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: target.ID, RefIssueID: source.ID, RefCommentID: p.ID, Type: issues_model.CommentTypeCommentRef})
	assert.Equal(t, "See #1", unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: source.ID}).Content)
	assert.Equal(t, parent.Content, p.Content)
}

func TestGithubMissingReplyFails(t *testing.T) {
	uploader := &githubUploader{replies: map[int64]int64{1: 2}, reviewComments: map[int64]*issues_model.Comment{1: {IssueID: 1}}}
	require.ErrorContains(t, uploader.Finish(t.Context()), "missing or unrelated parent")
}

func TestGithubTimelinePreservation(t *testing.T) {
	local := newGithubTestUploader(t)
	local.repo.Status = repo_model.RepositoryBeingMigrated
	require.NoError(t, resetGithubMigrationData(t.Context(), local.repo))
	require.NoError(t, local.CreateIssues(t.Context(), &base.Issue{Number: 405, Title: "Closed", State: "closed"}, &base.Issue{Number: 407, Title: "Open", State: "open"}))
	downloader := newGithubTestDownloader(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/o/r/issues/405/timeline":
			events := []*github.Timeline{}
			for id, number := range []int{407, 406} {
				events = append(events, &github.Timeline{ID: new(int64(10 + id)), Event: new("cross-referenced"), CreatedAt: &github.Timestamp{Time: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)}, Source: &github.Source{Issue: &github.Issue{Number: new(number), RepositoryURL: new("http://" + r.Host + "/api/v3/repos/o/r"), HTMLURL: new(fmt.Sprintf("https://github.com/o/r/issues/%d", number))}}})
			}
			assert.NoError(t, json.NewEncoder(w).Encode(events))
		case "/api/v3/repos/o/r/issues/407/timeline":
			_, _ = io.WriteString(w, "[]")
		default:
			t.Errorf("unexpected request: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}).GithubDownloaderV3
	archived := false
	downloader.archive = func(_ context.Context, kind string, _ int64, value any) error {
		if kind == "timelines/405" {
			events, ok := value.([]*github.Timeline)
			require.True(t, ok)
			require.Len(t, events, 2)
			assert.Equal(t, int64(10), events[0].GetID())
			archived = true
		}
		return nil
	}
	uploader := githubMigrationUploader(local, downloader, base.MigrateOptions{Issues: true, PullRequests: true})
	require.NoError(t, uploader.CreateComments(t.Context(), &base.Comment{Index: 1, IssueIndex: 405, Content: "Original comment"}))
	require.NoError(t, uploader.Finish(t.Context()))
	assert.True(t, archived)
	issue, target := local.issues[405], local.issues[407]
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Content: "Original comment"})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Content: "GitHub event: cross-referenced — [source](" + target.HTMLURL(t.Context()) + ")"})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Content: "GitHub event: cross-referenced — [source](https://github.com/o/r/issues/406)"})
	unittest.AssertNotExistsBean(t, &issues_model.Issue{RepoID: local.repo.ID, Index: 406})
	assert.Len(t, local.issues, 2)
	assert.True(t, issue.IsClosed)
	assert.False(t, target.IsClosed)
	assert.Equal(t, repo_model.RepositoryReady, local.repo.Status)
}

func TestGithubReferenceLinks(t *testing.T) {
	unittest.PrepareTestEnv(t)
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: 1, Index: 1})
	require.NoError(t, issue.LoadRepo(t.Context()))
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeComment})
	uploader := &githubUploader{GiteaLocalUploader: &GiteaLocalUploader{issues: map[int64]*issues_model.Issue{1: issue}}, downloader: &GithubDownloaderV3{baseURL: "https://github.com", repoOwner: "o", repoName: "r"}, comments: map[int64]*issues_model.Comment{42: comment}}
	for _, example := range []struct{ input, expected string }{
		{"https://github.com/o/r/issues/1", issue.HTMLURL(t.Context())},
		{"[ref](https://github.com/o/r/issues/1#issuecomment-42)", "[ref](" + comment.HTMLURL(t.Context()) + ")"},
		{"https://github.com/o/r/issues/123", "https://github.com/o/r/issues/123"},
		{"https://github.com/o/r/issues/1#issuecomment-99", "https://github.com/o/r/issues/1#issuecomment-99"},
		{"https://github.com/other/r/issues/1", "https://github.com/other/r/issues/1"},
	} {
		assert.Equal(t, example.expected, uploader.rewriteLinks(t.Context(), example.input))
	}
}

func TestGithubMigrationPullStates(t *testing.T) {
	checks := 0
	defer test.MockVariableValue(&pull.AddPullRequestToCheckQueue, func(int64) { checks++ })()
	local := newGithubTestUploader(t)
	openGithubTestRepository(t, local)
	sha, err := local.gitRepo.GetRefCommitID(t.Context(), "HEAD")
	require.NoError(t, err)
	downloader, err := NewGithubDownloaderV3(t.Context(), "https://github.com", "", "", "", "o", "r")
	require.NoError(t, err)
	downloader.SkipReactions = true
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = io.WriteString(w, "patch")
	}))
	defer server.Close()
	uploader := &githubUploader{GiteaLocalUploader: local}
	created := github.Timestamp{Time: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)}
	closed := github.Timestamp{Time: created.Add(time.Hour)}
	for i, example := range []struct {
		name, state   string
		merged, draft bool
	}{
		{name: "open", state: "open"},
		{name: "closed", state: "closed"},
		{name: "merged", state: "closed", merged: true},
		{name: "draft", state: "open", draft: true},
	} {
		t.Run(example.name, func(t *testing.T) {
			source := &github.PullRequest{
				Number: new(9000 + i), Title: new("Source PR"), State: new(example.state), Draft: new(example.draft), Locked: new(true),
				CreatedAt: &created, UpdatedAt: &created,
				Head: &github.PullRequestBranch{SHA: new(sha), Ref: new("source")},
				Base: &github.PullRequestBranch{SHA: new(sha), Ref: new("master")},
			}
			if example.state == "closed" {
				source.ClosedAt = &closed
			}
			if example.merged {
				source.MergedAt, source.MergeCommitSHA = &closed, new(sha)
			}
			prs, err := downloader.convertGithubPullRequests(t.Context(), []*github.PullRequest{source}, 100)
			require.NoError(t, err)
			prs[0].PatchURL = server.URL
			previousChecks := checks
			require.NoError(t, uploader.CreatePullRequests(t.Context(), prs...))
			if example.state == "closed" {
				assert.Equal(t, previousChecks, checks)
			}
			assert.Equal(t, server.URL, prs[0].PatchURL)
			assert.Equal(t, i+1, requests)
			patch, err := os.ReadFile(filepath.Join(gitrepo.RepoLocalPath(local.repo), fmt.Sprintf("pulls/%d.patch", prs[0].Number)))
			require.NoError(t, err)
			assert.Equal(t, "patch", string(patch))
			head, err := local.gitRepo.GetRefCommitID(t.Context(), prs[0].GetGitHeadRefName())
			require.NoError(t, err)
			assert.Equal(t, sha, head)
			issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: local.repo.ID, Index: int64(source.GetNumber())})
			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: local.repo.ID, Index: issue.Index})
			assert.Equal(t, example.state == "closed", issue.IsClosed)
			assert.Equal(t, example.merged, pr.HasMerged)
			assert.Equal(t, example.draft, issues_model.HasWorkInProgressPrefix(issue.Title))
			assert.True(t, issue.IsLocked)
			assert.Equal(t, created.Unix(), int64(issue.CreatedUnix))
			if example.state == "closed" {
				assert.Equal(t, closed.Unix(), int64(issue.ClosedUnix))
			}
			if example.merged {
				assert.Equal(t, closed.Unix(), int64(pr.MergedUnix))
				assert.Equal(t, sha, pr.MergedCommitID)
			}
		})
	}
	preserved, err := local.gitRepo.GetRefCommitID(t.Context(), "refs/migration/github/commits/"+sha)
	require.NoError(t, err)
	assert.Equal(t, sha, preserved)
	assert.Len(t, uploader.preservedCommits, 1)
	refPath := filepath.Join(gitrepo.RepoLocalPath(local.repo), "refs/migration/github/commits", sha)
	lock, err := os.Create(refPath + ".lock")
	require.NoError(t, err)
	require.NoError(t, lock.Close())
	defer os.Remove(refPath + ".lock")
	require.NoError(t, uploader.preserveCommit(t.Context(), sha))
	uncached := &githubUploader{GiteaLocalUploader: local}
	require.Error(t, uncached.preserveCommit(t.Context(), sha))
	assert.Empty(t, uncached.preservedCommits)
	require.NoError(t, os.Remove(refPath+".lock"))
	require.NoError(t, uncached.preserveCommit(t.Context(), sha))
	assert.Contains(t, uncached.preservedCommits, sha)
	pr := &base.PullRequest{Number: 4878, EnsuredSafe: true, Base: base.PullRequestBranch{SHA: sha}}
	require.ErrorContains(t, uploader.CreatePullRequests(t.Context(), pr), "has no head or base commit")
	unittest.AssertNotExistsBean(t, &issues_model.PullRequest{BaseRepoID: local.repo.ID, Index: pr.Number})
}

func TestGithubMigrationUploaderIsolation(t *testing.T) {
	local := &GiteaLocalUploader{}
	github := &GithubDownloaderV3{}
	for _, downloader := range []base.Downloader{
		github, newGithubGraphQLDownloader(github, false), &GitlabDownloader{}, &GiteaDownloader{}, &GogsDownloader{},
		&GitBucketDownloader{}, &OneDevDownloader{}, &CodebaseDownloader{}, &CodeCommitDownloader{}, &PlainGitDownloader{},
	} {
		for _, wrapped := range []base.Downloader{downloader, base.NewRetryDownloader(downloader, 2, 0)} {
			uploader := githubMigrationUploader(local, wrapped, base.MigrateOptions{Issues: true})
			switch downloader.(type) {
			case *GithubDownloaderV3, *GithubGraphQLDownloader:
				adapter, ok := uploader.(*githubUploader)
				require.True(t, ok)
				assert.Same(t, local, adapter.GiteaLocalUploader)
				assert.Same(t, github, adapter.downloader)
			default:
				assert.Same(t, local, uploader)
			}
		}
	}
}

func TestGithubMigrationRejectsUnsafePatch(t *testing.T) {
	uploader := &githubUploader{GiteaLocalUploader: &GiteaLocalUploader{}}
	err := uploader.CreatePullRequests(t.Context(), &base.PullRequest{Number: 1, PatchURL: "http://127.0.0.1/patch"})
	require.ErrorContains(t, err, "was not checked for safety")
}

func TestGithubPullHeadFetchRewrittenHistory(t *testing.T) {
	defer test.MockVariableValue(&pull.AddPullRequestToCheckQueue, func(int64) {})()
	local := newGithubTestUploader(t)
	openGithubTestRepository(t, local)
	baseSHA, err := local.gitRepo.GetRefCommitID(t.Context(), "HEAD")
	require.NoError(t, err)
	localDir := gitrepo.RepoLocalPath(local.repo)
	remoteDir := filepath.Join(t.TempDir(), "source.git")
	require.NoError(t, gitcmd.NewCommand("clone", "--bare").AddDynamicArguments(localDir, remoteDir).Run(t.Context()))
	tree, _, err := gitcmd.NewCommand("rev-parse", "HEAD^{tree}").WithDir(remoteDir).RunStdString(t.Context())
	require.NoError(t, err)
	commit := func(dir, message string) string {
		t.Helper()
		sha, _, err := gitcmd.NewCommand("commit-tree").AddDynamicArguments(strings.TrimSpace(tree)).AddArguments("-p").AddDynamicArguments(baseSHA).AddArguments("-m").AddDynamicArguments(message).AddConfig("user.name", "test").AddConfig("user.email", "test@example.com").WithDir(dir).RunStdString(t.Context())
		require.NoError(t, err)
		return strings.TrimSpace(sha)
	}
	if _, err := git.GetRemoteAddress(t.Context(), local.repo, "origin"); err == nil {
		require.NoError(t, git.ManagedRemoteRemove(t.Context(), local.repo, "origin"))
	} else {
		require.True(t, git.IsRemoteNotExistError(err))
	}
	oldHead := commit(localDir, "old head")
	newHead := commit(remoteDir, "rewritten head")
	pr := &base.PullRequest{Number: 11391, Title: "Rewritten PR", State: "open", EnsuredSafe: true, Head: base.PullRequestBranch{SHA: newHead, Ref: "deleted-fork-branch", OwnerName: "fork", RepoName: "r", CloneURL: "https://unavailable.invalid/fork.git"}, Base: base.PullRequestBranch{SHA: baseSHA, Ref: "master", OwnerName: "o", RepoName: "r", CloneURL: "https://github.com/o/r.git"}}
	require.NoError(t, git.UpdateRef(t.Context(), local.repo, pr.GetGitHeadRefName(), oldHead))
	require.NoError(t, gitcmd.NewCommand("update-ref").AddDynamicArguments(pr.GetGitHeadRefName(), newHead).WithDir(remoteDir).Run(t.Context()))
	_, err = local.gitRepo.GetCommit(t.Context(), newHead)
	require.Error(t, err)
	uploader := &githubUploader{GiteaLocalUploader: local, sourceCloneURL: remoteDir}
	require.NoError(t, uploader.CreatePullRequests(t.Context(), pr))
	head, err := local.gitRepo.GetRefCommitID(t.Context(), pr.GetGitHeadRefName())
	require.NoError(t, err)
	assert.Equal(t, newHead, head)
	row := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: local.repo.ID, Index: pr.Number})
	assert.NotEqual(t, "unknown repository", row.HeadBranch)
	branchSHA, err := local.gitRepo.GetBranchCommitID(t.Context(), row.HeadBranch)
	require.NoError(t, err)
	assert.Equal(t, newHead, branchSHA)
	pr.Number++
	require.NoError(t, gitcmd.NewCommand("update-ref").AddDynamicArguments(pr.GetGitHeadRefName(), newHead).WithDir(remoteDir).Run(t.Context()))
	require.NoError(t, uploader.CreatePullRequests(t.Context(), pr))
	head, err = local.gitRepo.GetRefCommitID(t.Context(), pr.GetGitHeadRefName())
	require.NoError(t, err)
	assert.Equal(t, newHead, head)
	sourceCommit := commit(remoteDir, "source-only commit")
	require.NoError(t, gitcmd.NewCommand("update-ref").AddDynamicArguments("refs/heads/source-only", sourceCommit).WithDir(remoteDir).Run(t.Context()))
	require.NoError(t, uploader.preserveCommit(t.Context(), sourceCommit))
	preserved, err := local.gitRepo.GetRefCommitID(t.Context(), "refs/migration/github/commits/"+sourceCommit)
	require.NoError(t, err)
	assert.Equal(t, sourceCommit, preserved)
}

func TestGithubMigrationPatchRetry(t *testing.T) {
	defer test.MockVariableValue(&setting.RepoRootPath, t.TempDir())()
	defer test.MockVariableValue(&setting.Migrations.MaxAttempts, 3)()
	defer test.MockVariableValue(&setting.Migrations.RetryBackoff, 0)()
	for _, tc := range []struct {
		name             string
		status, attempts int
	}{
		{name: "recover", attempts: 3},
		{name: "exhaust", attempts: 3},
		{name: "cancel", attempts: 1},
		{name: "forbidden", status: http.StatusForbidden, attempts: 3},
		{name: "not found", status: http.StatusNotFound, attempts: 3},
		{name: "server error", status: http.StatusInternalServerError, attempts: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := requests.Add(1)
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, "error page")
					return
				}
				if tc.name == "cancel" {
					cancel()
					<-r.Context().Done()
					return
				}
				if attempt == 1 {
					conn, _, err := http.NewResponseController(w).Hijack()
					if assert.NoError(t, err) {
						_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length:")
						_ = conn.Close()
					}
					return
				}
				if attempt == 2 || tc.name == "exhaust" {
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, "incomplete patch contents")
					return
				}
				_, _ = io.WriteString(w, "patch")
			}))
			defer server.Close()
			uploader := &githubUploader{GiteaLocalUploader: &GiteaLocalUploader{repo: &repo_model.Repository{OwnerName: "owner", Name: tc.name}}}
			err := retryGithubMigration(ctx, func() error {
				return uploader.downloadPatch(ctx, &base.PullRequest{Number: 1, PatchURL: server.URL, EnsuredSafe: true})
			})
			switch tc.name {
			case "recover":
				require.NoError(t, err)
				data, err := os.ReadFile(filepath.Join(gitrepo.RepoLocalPath(uploader.repo), "pulls/1.patch"))
				require.NoError(t, err)
				assert.Equal(t, "patch", string(data))
			case "exhaust":
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)
			case "cancel":
				require.ErrorIs(t, err, context.Canceled)
			default:
				require.ErrorContains(t, err, fmt.Sprintf("HTTP %d", tc.status))
				_, err := os.Stat(filepath.Join(gitrepo.RepoLocalPath(uploader.repo), "pulls/1.patch"))
				require.True(t, os.IsNotExist(err))
			}
			assert.EqualValues(t, tc.attempts, requests.Load())
		})
	}
}

func TestResetGithubMigrationData(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.Error(t, resetGithubMigrationData(ctx, repo))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: 1})

	repo.Status = repo_model.RepositoryBeingMigrated
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "status"))
	issues, err := issues_model.Issues(ctx, &issues_model.IssuesOptions{RepoIDs: []int64{repo.ID}})
	require.NoError(t, err)
	require.NotEmpty(t, issues)
	for range 2 {
		require.NoError(t, resetGithubMigrationData(ctx, repo))
		for _, bean := range []any{
			&repo_model.Release{RepoID: repo.ID},
			&repo_model.Attachment{RepoID: repo.ID},
			&issues_model.Issue{RepoID: repo.ID},
			&issues_model.PullRequest{BaseRepoID: repo.ID},
			&issues_model.Label{RepoID: repo.ID},
			&issues_model.Milestone{RepoID: repo.ID},
		} {
			unittest.AssertNotExistsBean(t, bean)
		}
		for _, issue := range issues {
			unittest.AssertNotExistsBean(t, &issues_model.Review{IssueID: issue.ID})
			unittest.AssertNotExistsBean(t, &issues_model.Comment{IssueID: issue.ID})
			unittest.AssertNotExistsBean(t, &issues_model.Reaction{IssueID: issue.ID})
			unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: issue.ID})
		}
		require.NoError(t, repo_model.InsertReleases(ctx, &repo_model.Release{RepoID: repo.ID, TagName: "v1.1"}))
		require.NoError(t, issues_model.InsertIssues(ctx, &issues_model.Issue{RepoID: repo.ID, Index: 1}))
	}
	unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: 2, RepoID: 40})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4, RepoID: 2})
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
	assert.Equal(t, repo_model.RepositoryBeingMigrated, repo.Status)
	assert.Zero(t, repo.NumMilestones)
	assert.Zero(t, repo.NumIssues)
}
