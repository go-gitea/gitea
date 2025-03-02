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

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func testCreateBranch(t testing.TB, session *TestSession, user, repo, oldRefSubURL, newBranchName string, expectedStatus int) string {
	csrf := GetUserCSRFToken(t, session)
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
	resp := session.MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Invalid CSRF token")
}

func prepareRecentlyPushedBranchTest(t *testing.T, headSession *TestSession, baseRepo, headRepo *repo_model.Repository) {
	refSubURL := fmt.Sprintf("branch/%s", headRepo.DefaultBranch)
	baseRepoPath := baseRepo.OwnerName + "/" + baseRepo.Name
	headRepoPath := headRepo.OwnerName + "/" + headRepo.Name
	// Case 1: Normal branch changeset to display pushed message
	// create branch with no new commit
	testCreateBranch(t, headSession, headRepo.OwnerName, headRepo.Name, refSubURL, "no-commit", http.StatusSeeOther)

	// create branch with commit
	testAPINewFile(t, headSession, headRepo.OwnerName, headRepo.Name, "new-commit", fmt.Sprintf("new-file-%s.txt", headRepo.Name), "new-commit")

	// create a branch then delete it
	testCreateBranch(t, headSession, headRepo.OwnerName, headRepo.Name, "branch/new-commit", "deleted-branch", http.StatusSeeOther)
	testUIDeleteBranch(t, headSession, headRepo.OwnerName, headRepo.Name, "deleted-branch")

	// only `new-commit` branch has commits ahead the base branch
	checkRecentlyPushedNewBranches(t, headSession, headRepoPath, []string{"new-commit"})
	if baseRepo.RepoPath() != headRepo.RepoPath() {
		checkRecentlyPushedNewBranches(t, headSession, baseRepoPath, []string{fmt.Sprintf("%v:new-commit", headRepo.FullName())})
	}

	// Case 2: Create PR so that `new-commit` branch will not show
	testCreatePullToDefaultBranch(t, headSession, baseRepo, headRepo, "new-commit", "merge new-commit to default branch")
	// No push message show because of active PR
	checkRecentlyPushedNewBranches(t, headSession, headRepoPath, []string{})
	if baseRepo.RepoPath() != headRepo.RepoPath() {
		checkRecentlyPushedNewBranches(t, headSession, baseRepoPath, []string{})
	}
}

func prepareRecentlyPushedBranchSpecialTest(t *testing.T, session *TestSession, baseRepo, headRepo *repo_model.Repository) {
	refSubURL := fmt.Sprintf("branch/%s", headRepo.DefaultBranch)
	baseRepoPath := baseRepo.OwnerName + "/" + baseRepo.Name
	headRepoPath := headRepo.OwnerName + "/" + headRepo.Name
	// create branch with no new commit
	testCreateBranch(t, session, headRepo.OwnerName, headRepo.Name, refSubURL, "no-commit-special", http.StatusSeeOther)

	// update base (default) branch before head branch is updated
	testAPINewFile(t, session, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, fmt.Sprintf("new-file-special-%s.txt", headRepo.Name), "new-commit")

	// Though we have new `no-commit` branch, but the headBranch is not newer or commits ahead baseBranch. No message show.
	checkRecentlyPushedNewBranches(t, session, headRepoPath, []string{})
	if baseRepo.RepoPath() != headRepo.RepoPath() {
		checkRecentlyPushedNewBranches(t, session, baseRepoPath, []string{})
	}
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
	refSubURL := fmt.Sprintf("branch/%s", headRepo.DefaultBranch)
	testCreateBranch(t, headSession, headRepo.OwnerName, headRepo.Name, refSubURL, "new-commit", http.StatusSeeOther)

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
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user12Session := loginUser(t, "user12")

		// Same reposioty check
		repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
		prepareRepoPR(t, user12Session, user12Session, repo10, repo10)
		prepareRecentlyPushedBranchTest(t, user12Session, repo10, repo10)
		prepareRecentlyPushedBranchSpecialTest(t, user12Session, repo10, repo10)

		// create a fork repo in public org
		testRepoFork(t, user12Session, repo10.OwnerName, repo10.Name, "org25", "org25_fork_repo10", repo10.DefaultBranch)
		orgPublicForkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: 25, Name: "org25_fork_repo10"})
		prepareRepoPR(t, user12Session, user12Session, repo10, orgPublicForkRepo)
		prepareRecentlyPushedBranchTest(t, user12Session, repo10, orgPublicForkRepo)
		prepareRecentlyPushedBranchSpecialTest(t, user12Session, repo10, orgPublicForkRepo)
	})
}
