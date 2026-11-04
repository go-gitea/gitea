// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type gerritFixture struct {
	*httptest.Server
	htmlCalls    atomic.Int32
	commentCalls atomic.Int32
}

func writeGerritJSON(w http.ResponseWriter, v any) {
	data, _ := json.Marshal(v)
	_, _ = w.Write(append([]byte(")]}'\n"), data...))
}

func newGerritFixture(t *testing.T) *gerritFixture {
	f := &gerritFixture{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() == "/projects/gerrit%2Fteam%2Fdemo" {
			f.htmlCalls.Add(1)
			_, _ = w.Write([]byte("<html>not a Gerrit</html>"))
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/gerrit")
		if rest, isAuth := strings.CutPrefix(path, "/a/"); isAuth {
			if user, password, _ := r.BasicAuth(); user != "alice" || password != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			path = "/" + rest
		}
		switch {
		case path == "/projects/team/demo":
			writeGerritJSON(w, map[string]any{"name": "team/demo", "description": "demo project"})
		case path == "/projects/team/demo/HEAD":
			writeGerritJSON(w, "refs/heads/main")
		case path == "/changes/":
			start, _ := strconv.Atoi(r.URL.Query().Get("S"))
			writeGerritJSON(w, []map[string]any{{"_number": start + 1, "_more_changes": start < 2}})
		case strings.HasSuffix(path, "/comments"):
			f.commentCalls.Add(1)
			writeGerritJSON(w, map[string]any{})
		case strings.HasPrefix(path, "/changes/team/demo~"):
			number, _ := strconv.Atoi(strings.TrimPrefix(path, "/changes/team/demo~"))
			writeGerritJSON(w, map[string]any{"_number": number})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func newTestGerritDownloader(t *testing.T, cloneAddr string, opts base.MigrateOptions) *GerritDownloader {
	t.Helper()
	opts.CloneAddr = cloneAddr
	downloader, err := (&GerritDownloaderFactory{}).New(t.Context(), opts)
	require.NoError(t, err)
	d, ok := downloader.(*GerritDownloader)
	require.True(t, ok)
	return d
}

func TestGerritCloneAddr(t *testing.T) {
	for _, addr := range []string{
		"ssh://alice@gerrit.example.com:29418/team/demo",
		"git@gerrit.example.com:team/demo.git",
		"git://gerrit.example.com/team/demo",
		"file:///srv/git/demo",
		"https:///team/demo",
		"https://gerrit.example.com/",
		"https://gerrit.example.com/team/../demo",
	} {
		_, err := parseGerritCloneAddr(addr)
		require.Error(t, err, addr)
	}
	_, err := parseGerritCloneAddr("ssh://alice@gerrit.example.com:29418/team/demo")
	require.ErrorContains(t, err, "unsupported scheme")
	_, err = parseGerritCloneAddr("https://gerrit.example.com/a/team/demo.git")
	require.NoError(t, err)

	_, err = parseGerritCloneAddr("https://alice:secret@gerrit.example.com/")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")

	location, err := parseGerritCloneAddr("https://alice:secret@gerrit.example.com/r/a/team/demo.git")
	require.NoError(t, err)
	assert.Equal(t, "alice", location.username)
	assert.Equal(t, "secret", location.password)
	assert.NotContains(t, location.String(), "secret")
}

func TestGerritRepoInfoAndResolve(t *testing.T) {
	f := newGerritFixture(t)
	withCredentials := strings.Replace(f.URL, "http://", "http://alice:secret@", 1)

	d := newTestGerritDownloader(t, f.URL+"/gerrit/team/demo.git", base.MigrateOptions{})
	repo, err := d.GetRepoInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, &base.Repository{
		Name:          "demo",
		Description:   "demo project",
		CloneURL:      f.URL + "/gerrit/team/demo",
		OriginalURL:   f.URL + "/gerrit/admin/repos/team%2Fdemo",
		DefaultBranch: "main",
	}, repo)
	assert.EqualValues(t, 1, f.htmlCalls.Load(), "the HTML answer of the root candidate is skipped")

	d = newTestGerritDownloader(t, f.URL+"/gerrit/a/team/demo", base.MigrateOptions{AuthUsername: "alice", AuthPassword: "secret"})
	repo, err = d.GetRepoInfo(t.Context())
	require.NoError(t, err)
	cloneURL, err := d.FormatCloneURL(base.MigrateOptions{}, repo.CloneURL)
	require.NoError(t, err)
	assert.Equal(t, withCredentials+"/gerrit/a/team/demo", cloneURL)

	d = newTestGerritDownloader(t, withCredentials+"/gerrit/team/demo.git", base.MigrateOptions{})
	_, err = d.GetRepoInfo(t.Context())
	require.NoError(t, err)
	assert.NotContains(t, d.String()+d.LogString(), "secret")

	d = newTestGerritDownloader(t, f.URL+"/gerrit/team/demo.git", base.MigrateOptions{AuthUsername: "alice", AuthPassword: "wrong"})
	_, err = d.GetRepoInfo(t.Context())
	assert.True(t, IsAuthenticationError(err), err)

	d = newTestGerritDownloader(t, f.URL+"/gerrit/team/other", base.MigrateOptions{})
	_, err = d.GetRepoInfo(t.Context())
	require.ErrorContains(t, err, "no Gerrit project found")
}

func TestGerritActivityCache(t *testing.T) {
	f := newGerritFixture(t)
	d := newTestGerritDownloader(t, f.URL+"/gerrit/team/demo.git", base.MigrateOptions{})
	ctx := t.Context()
	_, err := d.GetRepoInfo(ctx)
	require.NoError(t, err)
	prs, isEnd, err := d.GetPullRequests(ctx, 1, 2)
	require.NoError(t, err)
	require.Len(t, prs, 2)
	assert.False(t, isEnd)

	for _, pr := range prs {
		_, _, err := d.GetComments(ctx, pr)
		require.NoError(t, err)
		_, err = d.GetReviews(ctx, pr)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 2, f.commentCalls.Load(), "the comments of each change are requested once")
}

func TestParseGerritVotes(t *testing.T) {
	patchSet, votes := parseGerritVotes("Patch Set 12: Code-Review-2 Verified+1 -Foo\n\ntext Code-Review+2")
	assert.Equal(t, 12, patchSet)
	assert.Equal(t, []gerritVote{{"Code-Review", -2}, {"Verified", 1}}, votes)
	assert.Equal(t, base.ReviewStateChangesRequested, gerritReviewState(votes))
	assert.Equal(t, base.ReviewStateCommented, gerritReviewState([]gerritVote{{"Code-Review", 1}, {"Verified", 1}}))

	patchSet, votes = parseGerritVotes("Just a message")
	assert.Zero(t, patchSet)
	assert.Empty(t, votes)
}

// TestGerritRecordedResponses replays raw responses of https://gerrit-review.googlesource.com, project "gerrit",
// change 646943. Each fixture is a recorded body, the XSSI prefix included, with the Content-Type header, unmodified.
// To refresh them, pass true as the last argument of NewMockWebServer to record, then restore false to replay.
func TestGerritRecordedResponses(t *testing.T) {
	_, callerFile, _, _ := runtime.Caller(0)
	dataDir := filepath.Join(filepath.Dir(callerFile), "_mock_data", "TestGerritRecordedResponses")
	server := unittest.NewMockWebServer(t, "https://gerrit-review.googlesource.com", dataDir, false)

	ctx := t.Context()
	d := newTestGerritDownloader(t, server.URL+"/gerrit", base.MigrateOptions{})

	repo, err := d.GetRepoInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "gerrit", repo.Name)
	assert.Equal(t, "Gerrit Code Review", repo.Description)
	assert.Equal(t, "master", repo.DefaultBranch)

	pr, err := d.getPullRequest(ctx, 646943)
	require.NoError(t, err)
	assert.EqualValues(t, 646943, pr.Number)
	assert.Equal(t, "Bazel: Build in-tree plugins against the unpublished plugin API", pr.Title)
	assert.Equal(t, "closed", pr.State)
	assert.True(t, pr.Merged)
	assert.False(t, pr.IsDraft)
	assert.Equal(t, "master", pr.Base.Ref)
	assert.Equal(t, "5d66555e5eaa3189f0b7956e7ff168110a5652ef", pr.Base.SHA)
	assert.Equal(t, "refs/changes/43/646943/3", pr.Head.Ref)
	assert.Equal(t, "497721cb3009183d3c38b0e5e3be5e839d0d690c", pr.Head.SHA)
	assert.EqualValues(t, 1011323, pr.PosterID)
	assert.Equal(t, "David Ostrovsky", pr.PosterName)
	assert.Contains(t, pr.Content, "Change-Id: I7ae8896425044789699051bcc06fd38cc528be47")
	assert.Equal(t, time.Date(2026, 10, 22, 10, 39, 24, 0, time.UTC), pr.MergedTime.UTC())

	change, ok := pr.Context.(*gerritChange)
	require.True(t, ok)
	assert.Len(t, change.Revisions, 3)
	assert.Equal(t, "56a6ac827af5d2180bc18ef5d7cf4aad8baf2cd4", change.revision(1))

	reviews, err := d.GetReviews(ctx, pr)
	require.NoError(t, err)
	states := map[string]int{}
	for _, review := range reviews {
		states[review.State]++
		if review.State == base.ReviewStateApproved {
			assert.EqualValues(t, 1025452, review.ReviewerID)
			assert.Equal(t, "497721cb3009183d3c38b0e5e3be5e839d0d690c", review.CommitID)
			assert.Equal(t, "Patch Set 3: Code-Review+2", review.Content)
		}
	}
	assert.Equal(t, map[string]int{base.ReviewStateApproved: 1, base.ReviewStateCommented: 9}, states)

	comments, _, err := d.GetComments(ctx, pr)
	require.NoError(t, err)
	var patchSetLevel, merged int
	for _, comment := range comments {
		switch {
		case strings.HasPrefix(comment.Content, "Patch set level comment (patch set 3, revision 497721cb"):
			patchSetLevel++
		case comment.Content == "Change has been successfully merged":
			merged++
		}
	}
	assert.Len(t, comments, 7)
	assert.Equal(t, 2, patchSetLevel)
	assert.Equal(t, 1, merged)
}
