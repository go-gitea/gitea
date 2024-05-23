// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func testCreateBranch(t testing.TB, session *TestSession, user, repo, oldRefSubURL, newBranchName string, expectedStatus int) string {
	var csrf string
	if expectedStatus == http.StatusNotFound {
		// src/branch/branch_name may not container "_csrf" input,
		// so we need to get it from cookies not from body
		csrf = GetCSRFFromCookie(t, session, path.Join(user, repo, "src/branch/master"))
	} else {
		csrf = GetCSRFFromCookie(t, session, path.Join(user, repo, "src", oldRefSubURL))
	}
	req := NewRequestWithValues(t, "POST", path.Join(user, repo, "branches/_new", oldRefSubURL), map[string]string{
		"_csrf":           csrf,
		"new_branch_name": newBranchName,
	})
	resp := session.MakeRequest(t, req, expectedStatus)
	if expectedStatus != http.StatusSeeOther {
		return ""
	}
	return test.RedirectURL(resp)
}

func TestCreateBranch(t *testing.T) {
	onGiteaRun(t, testCreateBranches)
}

func testCreateBranches(t *testing.T, giteaURL *url.URL) {
	tests := []struct {
		OldRefSubURL   string
		NewBranch      string
		CreateRelease  string
		FlashMessage   string
		ExpectedStatus int
	}{
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "feature/test1",
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("repo.branch.create_success", "feature/test1"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "",
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("form.NewBranchName") + translation.NewLocale("en-US").TrString("form.require_error"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "feature=test1",
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("repo.branch.create_success", "feature=test1"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      strings.Repeat("b", 101),
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("form.NewBranchName") + translation.NewLocale("en-US").TrString("form.max_size_error", "100"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "master",
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("repo.branch.branch_already_exists", "master"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "master/test",
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("repo.branch.branch_name_conflict", "master/test", "master"),
		},
		{
			OldRefSubURL:   "commit/acd1d892867872cb47f3993468605b8aa59aa2e0",
			NewBranch:      "feature/test2",
			ExpectedStatus: http.StatusNotFound,
		},
		{
			OldRefSubURL:   "commit/65f1bf27bc3bf70f64657658635e66094edbcb4d",
			NewBranch:      "feature/test3",
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("repo.branch.create_success", "feature/test3"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "v1.0.0",
			CreateRelease:  "v1.0.0",
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("repo.branch.tag_collision", "v1.0.0"),
		},
		{
			OldRefSubURL:   "tag/v1.0.0",
			NewBranch:      "feature/test4",
			CreateRelease:  "v1.0.1",
			ExpectedStatus: http.StatusSeeOther,
			FlashMessage:   translation.NewLocale("en-US").TrString("repo.branch.create_success", "feature/test4"),
		},
	}
	for _, test := range tests {
		session := loginUser(t, "user2")
		if test.CreateRelease != "" {
			createNewRelease(t, session, "/user2/repo1", test.CreateRelease, test.CreateRelease, false, false)
		}
		redirectURL := testCreateBranch(t, session, "user2", "repo1", test.OldRefSubURL, test.NewBranch, test.ExpectedStatus)
		if test.ExpectedStatus == http.StatusSeeOther {
			req := NewRequest(t, "GET", redirectURL)
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			assert.Contains(t,
				strings.TrimSpace(htmlDoc.doc.Find(".ui.message").Text()),
				test.FlashMessage,
			)
		}
	}
}

func TestCreateBranchInvalidCSRF(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "user2/repo1/branches/_new/branch/master", map[string]string{
		"_csrf":           "fake_csrf",
		"new_branch_name": "test",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	loc := resp.Header().Get("Location")
	assert.Equal(t, setting.AppSubURL+"/", loc)
	resp = session.MakeRequest(t, NewRequest(t, "GET", loc), http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Equal(t,
		"Bad Request: invalid CSRF token",
		strings.TrimSpace(htmlDoc.doc.Find(".ui.message").Text()),
	)
}

func prepareBranch(t *testing.T, session *TestSession, repo *repo_model.Repository) {
	baseRefSubURL := fmt.Sprintf("branch/%s", repo.DefaultBranch)

	// create branch with no new commit
	testCreateBranch(t, session, repo.OwnerName, repo.Name, baseRefSubURL, "no-commit", http.StatusSeeOther)

	// create branch with commit
	testCreateBranch(t, session, repo.OwnerName, repo.Name, baseRefSubURL, "new-commit", http.StatusSeeOther)
	testAPINewFile(t, session, repo.OwnerName, repo.Name, "new-commit", "new-commit.txt", "new-commit")

	// create deleted branch
	testCreateBranch(t, session, repo.OwnerName, repo.Name, "branch/new-commit", "deleted-branch", http.StatusSeeOther)
	testUIDeleteBranch(t, session, repo.OwnerName, repo.Name, "deleted-branch")
}

func testCreatePullToDefaultBranch(t *testing.T, session *TestSession, baseRepo, headRepo *repo_model.Repository, headBranch, title string) string {
	srcRef := headBranch
	if baseRepo.ID != headRepo.ID {
		srcRef = fmt.Sprintf("%s/%s:%s", headRepo.OwnerName, headRepo.Name, headBranch)
	}
	resp := testPullCreate(t, session, baseRepo.OwnerName, baseRepo.Name, false, baseRepo.DefaultBranch, srcRef, title)
	elem := strings.Split(test.RedirectURL(resp), "/")
	// return pull request ID
	return elem[4]
}

func prepareRepoPR(t *testing.T, baseSession, headSession *TestSession, baseRepo, headRepo *repo_model.Repository) {
	// create opening PR
	testCreateBranch(t, headSession, headRepo.OwnerName, headRepo.Name, "branch/new-commit", "opening-pr", http.StatusSeeOther)
	testCreatePullToDefaultBranch(t, baseSession, baseRepo, headRepo, "opening-pr", "opening pr")

	// create closed PR
	testCreateBranch(t, headSession, headRepo.OwnerName, headRepo.Name, "branch/new-commit", "closed-pr", http.StatusSeeOther)
	prID := testCreatePullToDefaultBranch(t, baseSession, baseRepo, headRepo, "closed-pr", "closed pr")
	testIssueClose(t, baseSession, baseRepo.OwnerName, baseRepo.Name, prID)

	// create closed PR with deleted branch
	testCreateBranch(t, headSession, headRepo.OwnerName, headRepo.Name, "branch/new-commit", "closed-pr-deleted", http.StatusSeeOther)
	prID = testCreatePullToDefaultBranch(t, baseSession, baseRepo, headRepo, "closed-pr-deleted", "closed pr with deleted branch")
	testIssueClose(t, baseSession, baseRepo.OwnerName, baseRepo.Name, prID)
	testUIDeleteBranch(t, headSession, headRepo.OwnerName, headRepo.Name, "closed-pr-deleted")

	// create merged PR
	testCreateBranch(t, headSession, headRepo.OwnerName, headRepo.Name, "branch/new-commit", "merged-pr", http.StatusSeeOther)
	prID = testCreatePullToDefaultBranch(t, baseSession, baseRepo, headRepo, "merged-pr", "merged pr")
	testAPINewFile(t, headSession, headRepo.OwnerName, headRepo.Name, "merged-pr", fmt.Sprintf("new-commit-%s.txt", headRepo.Name), "new-commit")
	testPullMerge(t, baseSession, baseRepo.OwnerName, baseRepo.Name, prID, repo_model.MergeStyleRebaseMerge, false)

	// create merged PR with deleted branch
	testCreateBranch(t, headSession, headRepo.OwnerName, headRepo.Name, "branch/new-commit", "merged-pr-deleted", http.StatusSeeOther)
	prID = testCreatePullToDefaultBranch(t, baseSession, baseRepo, headRepo, "merged-pr-deleted", "merged pr with deleted branch")
	testAPINewFile(t, headSession, headRepo.OwnerName, headRepo.Name, "merged-pr-deleted", fmt.Sprintf("new-commit-%s-2.txt", headRepo.Name), "new-commit")
	testPullMerge(t, baseSession, baseRepo.OwnerName, baseRepo.Name, prID, repo_model.MergeStyleRebaseMerge, true)
}

func checkRecentlyPushedNewBranches(t *testing.T, session *TestSession, repoPath string, expected []string) {
	branches := make([]string, 0, 2)
	req := NewRequest(t, "GET", repoPath)
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	doc.doc.Find(".ui.positive.message div a").Each(func(index int, branch *goquery.Selection) {
		branches = append(branches, branch.Text())
	})
	assert.Equal(t, expected, branches)
}

func TestRecentlyPushedNewBranches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1Session := loginUser(t, "user1")
		user2Session := loginUser(t, "user2")
		user12Session := loginUser(t, "user12")
		user13Session := loginUser(t, "user13")

		// prepare branch and PRs in original repo
		repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
		prepareBranch(t, user12Session, repo10)
		prepareRepoPR(t, user12Session, user12Session, repo10, repo10)

		// outdated new branch should not be displayed
		checkRecentlyPushedNewBranches(t, user12Session, "user12/repo10", []string{"new-commit"})

		// create a fork repo in public org
		testRepoFork(t, user12Session, repo10.OwnerName, repo10.Name, "org25", "org25_fork_repo10", "new-commit")
		orgPublicForkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: 25, Name: "org25_fork_repo10"})
		prepareRepoPR(t, user12Session, user12Session, repo10, orgPublicForkRepo)

		// user12 is the owner of the repo10 and the organization org25
		// in repo10, user12 has opening/closed/merged pr and closed/merged pr with deleted branch
		checkRecentlyPushedNewBranches(t, user12Session, "user12/repo10", []string{"org25/org25_fork_repo10:new-commit", "new-commit"})

		userForkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
		testCtx := NewAPITestContext(t, repo10.OwnerName, repo10.Name, auth_model.AccessTokenScopeWriteRepository)
		t.Run("AddUser13AsCollaborator", doAPIAddCollaborator(testCtx, "user13", perm.AccessModeWrite))
		prepareBranch(t, user13Session, userForkRepo)
		prepareRepoPR(t, user13Session, user13Session, repo10, userForkRepo)

		// create branch with same name in different repo by user13
		testCreateBranch(t, user13Session, repo10.OwnerName, repo10.Name, "branch/new-commit", "same-name-branch", http.StatusSeeOther)
		testCreateBranch(t, user13Session, userForkRepo.OwnerName, userForkRepo.Name, "branch/new-commit", "same-name-branch", http.StatusSeeOther)
		testCreatePullToDefaultBranch(t, user13Session, repo10, userForkRepo, "same-name-branch", "same name branch pr")

		// user13 pushed 2 branches with the same name in repo10 and repo11
		// and repo11's branch has a pr, but repo10's branch doesn't
		// in this case, we should get repo10's branch but not repo11's branch
		checkRecentlyPushedNewBranches(t, user13Session, "user12/repo10", []string{"same-name-branch", "user13/repo11:new-commit"})

		// create a fork repo in private org
		testRepoFork(t, user1Session, repo10.OwnerName, repo10.Name, "private_org35", "org35_fork_repo10", "new-commit")
		orgPrivateForkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: 35, Name: "org35_fork_repo10"})
		prepareRepoPR(t, user1Session, user1Session, repo10, orgPrivateForkRepo)

		// user1 is the owner of private_org35 and no write permission to repo10
		// so user1 can only see the branch in org35_fork_repo10
		checkRecentlyPushedNewBranches(t, user1Session, "user12/repo10", []string{"private_org35/org35_fork_repo10:new-commit"})

		// user2 push a branch in private_org35
		testCreateBranch(t, user2Session, orgPrivateForkRepo.OwnerName, orgPrivateForkRepo.Name, "branch/new-commit", "user-read-permission", http.StatusSeeOther)
		// convert write permission to read permission for code unit
		token := getTokenForLoggedInUser(t, user1Session, auth_model.AccessTokenScopeWriteOrganization)
		req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", 24), &api.EditTeamOption{
			Name:     "team24",
			UnitsMap: map[string]string{"repo.code": "read"},
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
		teamUnit := unittest.AssertExistsAndLoadBean(t, &org_model.TeamUnit{TeamID: 24, Type: unit.TypeCode})
		assert.Equal(t, perm.AccessModeRead, teamUnit.AccessMode)
		// user2 can see the branch as it is created by user2
		checkRecentlyPushedNewBranches(t, user2Session, "user12/repo10", []string{"private_org35/org35_fork_repo10:user-read-permission"})
	})
}
