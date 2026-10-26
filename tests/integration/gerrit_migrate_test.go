// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type gerritTestServer struct {
	*httptest.Server
	repoDir string
	base    string
	head    string
}

func runGerritTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=alice@example.com", "GIT_AUTHOR_DATE=2024-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=alice@example.com", "GIT_COMMITTER_DATE=2024-01-01T00:00:00Z")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// newGerritTestServer serves the project "team/demo" with REST API and git over HTTP, like a Gerrit under /gerrit
func newGerritTestServer(t *testing.T) *gerritTestServer {
	t.Helper()
	root := t.TempDir()
	s := &gerritTestServer{repoDir: filepath.Join(root, "team", "demo")}
	require.NoError(t, os.MkdirAll(filepath.Dir(s.repoDir), 0o755))

	work := t.TempDir()
	runGerritTestGit(t, work, "init", "-q", "-b", "main")
	write := func(content string) string {
		require.NoError(t, os.WriteFile(filepath.Join(work, "a.txt"), []byte(content), 0o644))
		runGerritTestGit(t, work, "add", "a.txt")
		runGerritTestGit(t, work, "commit", "-q", "-m", "commit "+content)
		return runGerritTestGit(t, work, "rev-parse", "HEAD")
	}
	s.base = write("l1\nl2\nl3\n")
	runGerritTestGit(t, work, "checkout", "-q", "-b", "change")
	patchSet1 := write("l1\nfirst\nl3\n")
	runGerritTestGit(t, work, "update-ref", "refs/changes/01/1/1", "HEAD")
	s.head = write("l1\nsecond\nl3\n")
	runGerritTestGit(t, work, "update-ref", "refs/changes/01/1/2", "HEAD")
	runGerritTestGit(t, work, "checkout", "-q", "main")
	runGerritTestGit(t, work, "clone", "-q", "--bare", work, s.repoDir)
	for _, ref := range []string{"refs/changes/01/1/1", "refs/changes/01/1/2"} {
		runGerritTestGit(t, work, "push", "-q", s.repoDir, ref+":"+ref)
	}

	execPath, err := exec.CommandContext(t.Context(), "git", "--exec-path").Output()
	require.NoError(t, err)
	gitBackend := &cgi.Handler{
		Path: filepath.Join(strings.TrimSpace(string(execPath)), "git-http-backend"),
		Root: "/gerrit",
		Env:  []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"},
	}

	account := func(id int, name string) map[string]any {
		return map[string]any{"_account_id": id, "name": name, "email": name + "@example.com"}
	}
	revision := func(number int, ref string) map[string]any {
		return map[string]any{
			"_number": number, "ref": ref, "created": "2024-02-01 10:00:00.000000000",
			"commit": map[string]any{"parents": []map[string]any{{"commit": s.base}}, "message": "Change one\n\nDescription\n\nChange-Id: I1\n"},
		}
	}
	change := map[string]any{
		"project": "team/demo", "branch": "main", "change_id": "I1", "subject": "Change one", "status": "NEW",
		"created": "2024-02-01 10:00:00.000000000", "updated": "2024-02-02 10:00:00.000000000",
		"owner": account(1000, "alice"), "_number": 1, "current_revision": s.head,
		"revisions": map[string]any{
			patchSet1: revision(1, "refs/changes/01/1/1"),
			s.head:    revision(2, "refs/changes/01/1/2"),
		},
		"messages": []map[string]any{
			{"id": "m1", "author": account(1001, "bob"), "date": "2024-02-02 10:00:00.000000000", "message": "Patch Set 2: Code-Review+2\n\nLooks good", "_revision_number": 2},
		},
	}
	comments := map[string][]map[string]any{
		"a.txt": {
			{"id": "c1", "patch_set": 2, "line": 2, "message": "nice", "updated": "2024-02-02 10:00:00.000000000", "author": account(1001, "bob")},
			{"id": "c2", "patch_set": 1, "line": 2, "message": "old", "updated": "2024-02-01 12:00:00.000000000", "author": account(1001, "bob")},
		},
	}
	writeJSON := func(w http.ResponseWriter, v any) {
		data, _ := json.Marshal(v)
		_, _ = w.Write([]byte(")]}'\n"))
		_, _ = w.Write(data)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch p := r.URL.Path; {
		case strings.HasPrefix(p, "/gerrit/team/demo/"):
			gitBackend.ServeHTTP(w, r)
		case p == "/gerrit/projects/team/demo":
			writeJSON(w, map[string]any{"name": "team/demo", "description": "from gerrit"})
		case p == "/gerrit/projects/team/demo/HEAD":
			writeJSON(w, "refs/heads/main")
		case p == "/gerrit/changes/":
			writeJSON(w, []map[string]any{{"_number": 1}})
		case p == "/gerrit/changes/team/demo~1":
			writeJSON(w, change)
		case p == "/gerrit/changes/team/demo~1/comments":
			writeJSON(w, comments)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *gerritTestServer) cloneAddr() string {
	return s.URL + "/gerrit/team/demo"
}

func TestGerritMigrateAPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)
	migrate := func(options *structs.MigrateRepoOptions, status int) *httptest.ResponseRecorder {
		options.RepoOwnerID = owner.ID
		options.Service = structs.GerritService.Name()
		return MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate", options).AddTokenAuth(token), status)
	}

	t.Run("SSHRejected", func(t *testing.T) {
		resp := migrate(&structs.MigrateRepoOptions{CloneAddr: "ssh://alice@gerrit.example.com:29418/team/demo", RepoName: "gerrit-ssh"}, http.StatusUnprocessableEntity)
		assert.Contains(t, resp.Body.String(), "SSH clone URLs are not supported")
		unittest.AssertNotExistsBean(t, &repo_model.Repository{Name: "gerrit-ssh"})
	})

	t.Run("Mirror", func(t *testing.T) {
		server := newGerritTestServer(t)
		migrate(&structs.MigrateRepoOptions{CloneAddr: server.cloneAddr(), RepoName: "gerrit-mirror", Mirror: true, MirrorInterval: "2h", PullRequests: true}, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "gerrit-mirror"})
		assert.True(t, repo.IsMirror)
		assert.Equal(t, "main", repo.DefaultBranch)
		mirror := unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: repo.ID})
		assert.Equal(t, 2*time.Hour, mirror.Interval)
		assert.Zero(t, unittest.GetCount(t, &issues_model.PullRequest{BaseRepoID: repo.ID}))
		assert.Zero(t, unittest.GetCount(t, &issues_model.Issue{RepoID: repo.ID}))

		gitRepo, err := git.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		defer gitRepo.Close()
		branches, _, err := gitRepo.GetBranchNames(t.Context(), 0, 0)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"main", "change"}, branches)
	})

	t.Run("MissingPatchSetRef", func(t *testing.T) {
		server := newGerritTestServer(t)
		runGerritTestGit(t, server.repoDir, "update-ref", "-d", "refs/changes/01/1/1")
		migrate(&structs.MigrateRepoOptions{CloneAddr: server.cloneAddr(), RepoName: "gerrit-broken", PullRequests: true}, http.StatusInternalServerError)
		broken := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "gerrit-broken"})
		assert.Zero(t, unittest.GetCount(t, &issues_model.Issue{RepoID: broken.ID}), "no pull request is created from an incomplete clone")
	})

	t.Run("Migrate", func(t *testing.T) {
		server := newGerritTestServer(t)
		migrate(&structs.MigrateRepoOptions{CloneAddr: server.cloneAddr(), RepoName: "gerrit-ok", PullRequests: true}, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "gerrit-ok"})
		assert.Equal(t, structs.GerritService, repo.OriginalServiceType)
		assert.False(t, repo.IsMirror)
		assert.Equal(t, "main", repo.DefaultBranch)

		gitRepo, err := git.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		defer gitRepo.Close()
		for _, ref := range []string{"refs/changes/01/1/1", "refs/changes/01/1/2"} {
			_, err := gitRepo.GetRefCommitID(t.Context(), ref)
			require.NoError(t, err)
		}

		pr, err := issues_model.GetPullRequestByIndex(t.Context(), repo.ID, 1)
		require.NoError(t, err)
		assert.Equal(t, "Change one", pr.Issue.Title)
		assert.Contains(t, pr.Issue.Content, "Change-Id: I1")
		assert.False(t, pr.Issue.IsClosed)
		assert.Equal(t, "alice", pr.Issue.OriginalAuthor)
		assert.Equal(t, owner.ID, pr.Issue.PosterID)
		assert.Equal(t, server.base, pr.MergeBase)
		headID, err := gitRepo.GetRefCommitID(t.Context(), pr.GetGitHeadRefName())
		require.NoError(t, err)
		assert.Equal(t, server.head, headID)

		codeComments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{IssueID: pr.IssueID, Type: issues_model.CommentTypeCode})
		require.NoError(t, err)
		var inline []string
		for _, c := range codeComments {
			inline = append(inline, fmt.Sprintf("%s:%d:%s", c.TreePath, c.Line, c.Content))
		}
		timelineComments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{IssueID: pr.IssueID, Type: issues_model.CommentTypeComment})
		require.NoError(t, err)
		var timeline []string
		for _, c := range timelineComments {
			timeline = append(timeline, c.Content)
		}
		assert.Equal(t, []string{"a.txt:2:nice"}, inline)
		require.Len(t, timeline, 1)
		assert.Contains(t, timeline[0], "patch set 1")
		assert.Contains(t, timeline[0], "old")
		reviews, err := issues_model.FindReviews(t.Context(), issues_model.FindReviewOptions{IssueID: pr.IssueID, Types: []issues_model.ReviewType{issues_model.ReviewTypeApprove}})
		require.NoError(t, err)
		require.Len(t, reviews, 1)
		assert.Equal(t, "bob", reviews[0].OriginalAuthor)
	})
}

func TestGerritMigrateWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	hasGerritEntry := func(doc *HTMLDoc) bool {
		return doc.Find(".migrate-entry-title").FilterFunction(func(_ int, s *goquery.Selection) bool {
			return strings.TrimSpace(s.Text()) == "Gerrit"
		}).Length() == 1
	}
	doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/repo/migrate"), http.StatusOK).Body)
	assert.True(t, hasGerritEntry(doc))
	doc = NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/repo/migrate?mirror=true"), http.StatusOK).Body)
	assert.True(t, hasGerritEntry(doc))

	resp := session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d", structs.GerritService)), http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)
	form := doc.Find("form.ui.form")
	assert.Equal(t, 1, form.Find(`input[name="clone_addr"]`).Length())
	assert.Equal(t, 1, form.Find(`input[name="auth_username"]`).Length())
	assert.Equal(t, 1, form.Find(`input[name="auth_password"]`).Length())
	assert.Equal(t, 1, form.Find(`input[name="auth_token"]`).Length())
	assert.Equal(t, 1, form.Find(`input[name="pull_requests"]`).Length())
	assert.Equal(t, 1, form.Find(`input[name="mirror"]`).Length())
	_, checked := form.Find(`input[name="mirror"]`).Attr("checked")
	assert.False(t, checked)
	assert.Zero(t, form.Find(`input[name="issues"]`).Length())

	resp = session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d&mirror=true", structs.GerritService)), http.StatusOK)
	_, checked = NewHTMLParser(t, resp.Body).Find(`input[name="mirror"]`).Attr("checked")
	assert.True(t, checked)

	link, _ := form.Attr("action")
	uid, _ := form.Find(`input[name="uid"]`).Attr("value")
	post := func(values map[string]string) string {
		values["uid"], values["service"] = uid, strconv.Itoa(int(structs.GerritService))
		values["repo_name"] = "gerrit-web"
		resp := session.MakeRequest(t, NewRequestWithValues(t, "POST", link, values), http.StatusBadRequest)
		return resp.Body.String()
	}
	assert.Contains(t, post(map[string]string{"clone_addr": "ssh://alice@gerrit.example.com:29418/team/demo"}), "SSH clone URLs are not supported")
	unittest.AssertNotExistsBean(t, &repo_model.Repository{Name: "gerrit-web"})
}
