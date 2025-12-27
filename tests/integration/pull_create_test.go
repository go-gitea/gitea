// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"sync"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git/gitcmd"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testPullCreate(t *testing.T, session *TestSession, user, repo string, toSelf bool, targetBranch, sourceBranch, title string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the PR button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#new-pull-request").Attr("href")
	assert.True(t, exists, "The template has changed")

	targetUser := strings.Split(link, "/")[1]
	if toSelf && targetUser != user {
		link = strings.Replace(link, targetUser, user, 1)
	}

	if targetBranch != "master" {
		link = strings.Replace(link, "master...", targetBranch+"...", 1)
	}
	if sourceBranch != "master" {
		if targetUser == user {
			link = strings.Replace(link, "...master", "..."+sourceBranch, 1)
		} else {
			link = strings.Replace(link, ":master", ":"+sourceBranch, 1)
		}
	}

	req = NewRequest(t, "GET", link)
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Submit the form for creating the pull
	htmlDoc = NewHTMLParser(t, resp.Body)
	link, exists = htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"title": title,
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	return resp
}

type createPullRequestOptions struct {
	BaseRepoOwner string
	BaseRepoName  string
	BaseBranch    string
	HeadRepoOwner string
	HeadRepoName  string
	HeadBranch    string
	Title         string
	ReviewerIDs   string // comma-separated list of user IDs
}

func (opts createPullRequestOptions) IsValid() bool {
	return opts.BaseRepoOwner != "" && opts.BaseRepoName != "" && opts.BaseBranch != "" &&
		opts.HeadBranch != "" && opts.Title != ""
}

func testPullCreateDirectly(t *testing.T, session *TestSession, opts createPullRequestOptions) *httptest.ResponseRecorder {
	if !opts.IsValid() {
		t.Fatal("Invalid pull request options")
	}

	headCompare := opts.HeadBranch
	if opts.HeadRepoOwner != "" {
		if opts.HeadRepoName != "" {
			headCompare = fmt.Sprintf("%s/%s:%s", opts.HeadRepoOwner, opts.HeadRepoName, opts.HeadBranch)
		} else {
			headCompare = fmt.Sprintf("%s:%s", opts.HeadRepoOwner, opts.HeadBranch)
		}
	}
	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/compare/%s...%s", opts.BaseRepoOwner, opts.BaseRepoName, opts.BaseBranch, headCompare))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Submit the form for creating the pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	params := map[string]string{
		"title": opts.Title,
	}
	if opts.ReviewerIDs != "" {
		params["reviewer_ids"] = opts.ReviewerIDs
	}
	req = NewRequestWithValues(t, "POST", link, params)
	resp = session.MakeRequest(t, req, http.StatusOK)
	return resp
}

func testPullCreateFailure(t *testing.T, session *TestSession, baseRepoOwner, baseRepoName, baseBranch, headRepoOwner, headRepoName, headBranch, title string) *httptest.ResponseRecorder {
	headCompare := headBranch
	if headRepoOwner != "" {
		if headRepoName != "" {
			headCompare = fmt.Sprintf("%s/%s:%s", headRepoOwner, headRepoName, headBranch)
		} else {
			headCompare = fmt.Sprintf("%s:%s", headRepoOwner, headBranch)
		}
	}
	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/compare/%s...%s", baseRepoOwner, baseRepoName, baseBranch, headCompare))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Submit the form for creating the pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"title": title,
	})
	resp = session.MakeRequest(t, req, http.StatusBadRequest)
	return resp
}

func TestPullCreate(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 3, repo1.NumPulls)
		assert.Equal(t, 3, repo1.NumOpenPulls)
		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		repo1 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 4, repo1.NumPulls)
		assert.Equal(t, 4, repo1.NumOpenPulls)

		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user2/repo1/pulls/[0-9]*$", url)

		// test create the pull request again and it should fail now
		link := "/user2/repo1/compare/master...user1/repo1:master"
		req := NewRequestWithValues(t, "POST", link, map[string]string{
			"title": "This is a pull title",
		})
		session.MakeRequest(t, req, http.StatusBadRequest)

		// check .diff can be accessed and matches performed change
		req = NewRequest(t, "GET", url+".diff")
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Regexp(t, `\+Hello, World \(Edited\)`, resp.Body)
		assert.Regexp(t, "^diff", resp.Body)
		assert.NotRegexp(t, "diff.*diff", resp.Body) // not two diffs, just one

		// check .patch can be accessed and matches performed change
		req = NewRequest(t, "GET", url+".patch")
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Regexp(t, `\+Hello, World \(Edited\)`, resp.Body)
		assert.Regexp(t, "diff", resp.Body)
		assert.Regexp(t, `Subject: \[PATCH\] Update README.md`, resp.Body)
		assert.NotRegexp(t, "diff.*diff", resp.Body) // not two diffs, just one
	})
}

func TestPullCreate_TitleEscape(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "<i>XSS PR</i>")

		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user2/repo1/pulls/[0-9]*$", url)

		// Edit title
		req := NewRequest(t, "GET", url)
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		editTestTitleURL, exists := htmlDoc.doc.Find(".issue-title-buttons button[data-update-url]").First().Attr("data-update-url")
		assert.True(t, exists, "The template has changed")

		req = NewRequestWithValues(t, "POST", editTestTitleURL, map[string]string{
			"title": "<u>XSS PR</u>",
		})
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", url)
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)
		titleHTML, err := htmlDoc.doc.Find(".comment-list .timeline-item.event .comment-text-line b").First().Html()
		assert.NoError(t, err)
		assert.Equal(t, "<strike>&lt;i&gt;XSS PR&lt;/i&gt;</strike>", titleHTML)
		titleHTML, err = htmlDoc.doc.Find(".comment-list .timeline-item.event .comment-text-line b").Next().Html()
		assert.NoError(t, err)
		assert.Equal(t, "&lt;u&gt;XSS PR&lt;/u&gt;", titleHTML)
	})
}

func testUIDeleteBranch(t *testing.T, session *TestSession, ownerName, repoName, branchName string) {
	relURL := "/" + path.Join(ownerName, repoName, "branches")
	req := NewRequestWithValues(t, "POST", relURL+"/delete", map[string]string{
		"name": branchName,
	})
	session.MakeRequest(t, req, http.StatusOK)
}

func testDeleteRepository(t *testing.T, session *TestSession, ownerName, repoName string) {
	relURL := "/" + path.Join(ownerName, repoName, "settings")
	req := NewRequestWithValues(t, "POST", relURL+"?action=delete", map[string]string{
		"repo_name": repoName,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
}

func TestPullBranchDelete(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer tests.PrepareTestEnv(t)()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testCreateBranch(t, session, "user1", "repo1", "branch/master", "master1", http.StatusSeeOther)
		testEditFile(t, session, "user1", "repo1", "master1", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master1", "This is a pull title")

		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user2/repo1/pulls/[0-9]*$", url)
		req := NewRequest(t, "GET", url)
		session.MakeRequest(t, req, http.StatusOK)

		// delete head branch and confirm pull page is ok
		testUIDeleteBranch(t, session, "user1", "repo1", "master1")
		req = NewRequest(t, "GET", url)
		session.MakeRequest(t, req, http.StatusOK)

		// delete head repository and confirm pull page is ok
		testDeleteRepository(t, session, "user1", "repo1")
		req = NewRequest(t, "GET", url)
		session.MakeRequest(t, req, http.StatusOK)
	})
}

/*
Setup:
The base repository is: user2/repo1
Fork repository to: user1/repo1
Push extra commit to: user2/repo1, which changes README.md
Create a PR on user1/repo1

Test checks:
Check if pull request can be created from base to the fork repository.
*/
func TestPullCreatePrFromBaseToFork(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		sessionFork := loginUser(t, "user1")
		testRepoFork(t, sessionFork, "user2", "repo1", "user1", "repo1", "")

		// Edit base repository
		sessionBase := loginUser(t, "user2")
		testEditFile(t, sessionBase, "user2", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		// Create a PR
		resp := testPullCreateDirectly(t, sessionFork, createPullRequestOptions{
			BaseRepoOwner: "user1",
			BaseRepoName:  "repo1",
			BaseBranch:    "master",
			HeadRepoOwner: "user2",
			HeadRepoName:  "repo1",
			HeadBranch:    "master",
			Title:         "This is a pull title",
		})
		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user1/repo1/pulls/[0-9]*$", url)
	})
}

func TestCreatePullRequestFromNestedOrgForks(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)

		const (
			baseOrg     = "test-fork-org1"
			midForkOrg  = "test-fork-org2"
			leafForkOrg = "test-fork-org3"
			repoName    = "test-fork-repo"
			patchBranch = "teabot-patch-1"
		)

		createOrg := func(name string) {
			req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &api.CreateOrgOption{
				UserName:   name,
				Visibility: "public",
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		createOrg(baseOrg)
		createOrg(midForkOrg)
		createOrg(leafForkOrg)

		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", baseOrg), &api.CreateRepoOption{
			Name:          repoName,
			AutoInit:      true,
			DefaultBranch: "main",
			Private:       false,
			Readme:        "Default",
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)
		var baseRepo api.Repository
		DecodeJSON(t, resp, &baseRepo)
		assert.Equal(t, "main", baseRepo.DefaultBranch)

		forkIntoOrg := func(srcOrg, dstOrg string) api.Repository {
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", srcOrg, repoName), &api.CreateForkOption{
				Organization: util.ToPointer(dstOrg),
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusAccepted)
			var forkRepo api.Repository
			DecodeJSON(t, resp, &forkRepo)
			assert.NotNil(t, forkRepo.Owner)
			if forkRepo.Owner != nil {
				assert.Equal(t, dstOrg, forkRepo.Owner.UserName)
			}
			return forkRepo
		}

		forkIntoOrg(baseOrg, midForkOrg)
		forkIntoOrg(midForkOrg, leafForkOrg)

		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", leafForkOrg, repoName, "patch-from-org3.txt"), &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				BranchName:    "main",
				NewBranchName: patchBranch,
				Message:       "create patch from org3",
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("patch content")),
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		prPayload := map[string]string{
			"head":  fmt.Sprintf("%s:%s", leafForkOrg, patchBranch),
			"base":  "main",
			"title": "test creating pull from test-fork-org3 to test-fork-org1",
		}
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/pulls", baseOrg, repoName), prPayload).AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusCreated)
		var pr api.PullRequest
		DecodeJSON(t, resp, &pr)
		assert.Equal(t, prPayload["title"], pr.Title)
		if assert.NotNil(t, pr.Head) {
			assert.Equal(t, patchBranch, pr.Head.Ref)
			if assert.NotNil(t, pr.Head.Repository) {
				assert.Equal(t, fmt.Sprintf("%s/%s", leafForkOrg, repoName), pr.Head.Repository.FullName)
			}
		}
		if assert.NotNil(t, pr.Base) {
			assert.Equal(t, "main", pr.Base.Ref)
			if assert.NotNil(t, pr.Base.Repository) {
				assert.Equal(t, fmt.Sprintf("%s/%s", baseOrg, repoName), pr.Base.Repository.FullName)
			}
		}
	})
}

func TestPullCreateParallel(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		sessionFork := loginUser(t, "user1")
		testRepoFork(t, sessionFork, "user2", "repo1", "user1", "repo1", "")

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 3, repo1.NumPulls)
		assert.Equal(t, 3, repo1.NumOpenPulls)

		var wg sync.WaitGroup
		for i := range 5 {
			wg.Go(func() {
				branchName := fmt.Sprintf("new-branch-%d", i)
				testEditFileToNewBranch(t, sessionFork, "user1", "repo1", "master", branchName, "README.md", fmt.Sprintf("Hello, World (Edited) %d\n", i))

				// Create a PR
				resp := testPullCreateDirectly(t, sessionFork, createPullRequestOptions{
					BaseRepoOwner: "user2",
					BaseRepoName:  "repo1",
					BaseBranch:    "master",
					HeadRepoOwner: "user1",
					HeadRepoName:  "repo1",
					HeadBranch:    branchName,
					Title:         fmt.Sprintf("This is a pull title %d", i),
				})
				// check the redirected URL
				url := test.RedirectURL(resp)
				assert.Regexp(t, "^/user2/repo1/pulls/[0-9]*$", url)
			})
		}
		wg.Wait()

		repo1 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 8, repo1.NumPulls)
		assert.Equal(t, 8, repo1.NumOpenPulls)
	})
}

func TestCreateAgitPullWithReadPermission(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		dstPath := t.TempDir()

		u.Path = "user2/repo1.git"
		u.User = url.UserPassword("user4", userPassword)

		doGitClone(dstPath, u)(t)
		doGitCheckoutWriteFileCommit(localGitAddCommitOptions{
			LocalRepoPath:   dstPath,
			CheckoutBranch:  "master",
			TreeFilePath:    "new-file-for-agit.txt",
			TreeFileContent: "temp content",
		})(t)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 3, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		err := gitcmd.NewCommand("push", "origin", "HEAD:refs/for/master", "-o").
			AddDynamicArguments("topic=test-topic").
			WithDir(dstPath).
			Run(t.Context())
		assert.NoError(t, err)

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 4, repo.NumOpenPulls)
	})
}

/*
Setup: user2 has repository, user1 forks it
---

1. User2 blocks User1
2. User1 adds changes to fork
3. User1 attempts to create a pull request
4. User1 sees alert that the action is not allowed because of the block
*/
func TestCreatePullWhenBlocked(t *testing.T) {
	RepoOwner := "user2"
	ForkOwner := "user16"
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// Setup
		// User1 forks repo1 from User2
		sessionFork := loginUser(t, ForkOwner)
		testRepoFork(t, sessionFork, RepoOwner, "repo1", ForkOwner, "forkrepo1", "")

		// 1. User2 blocks user1
		// sessionBase := loginUser(t, "user2")
		token := getUserToken(t, RepoOwner, auth_model.AccessTokenScopeWriteUser)

		req := NewRequest(t, "GET", "/api/v1/user/blocks/"+ForkOwner).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "PUT", "/api/v1/user/blocks/"+ForkOwner).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// 2. User1 adds changes to fork
		testEditFile(t, sessionFork, ForkOwner, "forkrepo1", "master", "README.md", "Hello, World (Edited)\n")

		// 3. User1 attempts to create a pull request
		testPullCreateFailure(t, sessionFork, RepoOwner, "repo1", "master", ForkOwner, "forkrepo1", "master", "This is a pull title")

		// Teardown
		// Unblock user
		req = NewRequest(t, "DELETE", "/api/v1/user/blocks/"+ForkOwner).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})
}
