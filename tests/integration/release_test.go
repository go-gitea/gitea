// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func createNewRelease(t *testing.T, session *TestSession, repoURL, tag, title string, preRelease, draft bool) {
	req := NewRequest(t, "GET", repoURL+"/releases/new")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")

	postData := map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"tag_name":   tag,
		"tag_target": "master",
		"title":      title,
		"content":    "",
	}
	if preRelease {
		postData["prerelease"] = "on"
	}
	if draft {
		postData["draft"] = "1"
	}
	req = NewRequestWithValues(t, "POST", link, postData)

	resp = session.MakeRequest(t, req, http.StatusSeeOther)

	test.RedirectURL(resp) // check that redirect URL exists
}

func checkLatestReleaseAndCount(t *testing.T, session *TestSession, repoURL, version, label string, count int) {
	req := NewRequest(t, "GET", repoURL+"/releases")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	labelText := htmlDoc.doc.Find("#release-list > li .detail .label").First().Text()
	assert.EqualValues(t, label, labelText)
	titleText := htmlDoc.doc.Find("#release-list > li .detail h4 a").First().Text()
	assert.EqualValues(t, version, titleText)

	releaseList := htmlDoc.doc.Find("#release-list > li")
	assert.EqualValues(t, count, releaseList.Length())
}

func TestViewReleases(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/releases")
	session.MakeRequest(t, req, http.StatusOK)

	// if CI is to slow this test fail, so lets wait a bit
	time.Sleep(time.Millisecond * 100)
}

func TestViewReleasesNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/releases")
	MakeRequest(t, req, http.StatusOK)
}

func TestCreateRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", false, false)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", translation.NewLocale("en-US").TrString("repo.release.stable"), 4)
}

func TestCreateReleasePreRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", true, false)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", translation.NewLocale("en-US").TrString("repo.release.prerelease"), 4)
}

func TestCreateReleaseDraft(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", false, true)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", translation.NewLocale("en-US").TrString("repo.release.draft"), 4)
}

func TestCreateReleasePaging(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	oldAPIDefaultNum := setting.API.DefaultPagingNum
	defer func() {
		setting.API.DefaultPagingNum = oldAPIDefaultNum
	}()
	setting.API.DefaultPagingNum = 10

	session := loginUser(t, "user2")
	// Create enough releases to have paging
	for i := 0; i < 12; i++ {
		version := fmt.Sprintf("v0.0.%d", i)
		createNewRelease(t, session, "/user2/repo1", version, version, false, false)
	}
	createNewRelease(t, session, "/user2/repo1", "v0.0.12", "v0.0.12", false, true)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.12", translation.NewLocale("en-US").TrString("repo.release.draft"), 10)

	// Check that user4 does not see draft and still see 10 latest releases
	session2 := loginUser(t, "user4")
	checkLatestReleaseAndCount(t, session2, "/user2/repo1", "v0.0.11", translation.NewLocale("en-US").TrString("repo.release.stable"), 10)
}

func TestViewReleaseListNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 57, OwnerName: "user2", LowerName: "repo-release"})

	link := repo.Link() + "/releases"

	req := NewRequest(t, "GET", link)
	rsp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	releases := htmlDoc.Find("#release-list .release-entry")
	assert.Equal(t, 5, releases.Length())

	links := make([]string, 0, 5)
	commitsToMain := make([]string, 0, 5)
	releases.Each(func(i int, s *goquery.Selection) {
		link, exist := s.Find(".release-list-title a").Attr("href")
		if !exist {
			return
		}
		links = append(links, link)

		commitsToMain = append(commitsToMain, s.Find(".ahead > a").Text())
	})

	assert.EqualValues(t, []string{
		"/user2/repo-release/releases/tag/empty-target-branch",
		"/user2/repo-release/releases/tag/non-existing-target-branch",
		"/user2/repo-release/releases/tag/v2.0",
		"/user2/repo-release/releases/tag/v1.1",
		"/user2/repo-release/releases/tag/v1.0",
	}, links)
	assert.EqualValues(t, []string{
		"1 commits", // like v1.1
		"1 commits", // like v1.1
		"0 commits",
		"1 commits", // should be 3 commits ahead and 2 commits behind, but not implemented yet
		"3 commits",
	}, commitsToMain)
}

func TestViewSingleRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("NoLogin", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo-release/releases/tag/v1.0")
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		// check the "number of commits to main since this release"
		releaseList := htmlDoc.doc.Find("#release-list .ahead > a")
		assert.EqualValues(t, 1, releaseList.Length())
		assert.EqualValues(t, "3 commits", releaseList.First().Text())
	})
	t.Run("Login", func(t *testing.T) {
		session := loginUser(t, "user1")
		req := NewRequest(t, "GET", "/user2/repo1/releases/tag/delete-tag") // "delete-tag" is the only one with is_tag=true (although strange name)
		resp := session.MakeRequest(t, req, http.StatusOK)
		// the New Release button should contain the tag name
		assert.Contains(t, resp.Body.String(), `<a class="ui small primary button" href="/user2/repo1/releases/new?tag=delete-tag">`)
	})
}

func TestViewReleaseListLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	link := repo.Link() + "/releases"

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", link)
	rsp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	releases := htmlDoc.Find("#release-list .release-entry")
	assert.Equal(t, 3, releases.Length())

	links := make([]string, 0, 5)
	releases.Each(func(i int, s *goquery.Selection) {
		link, exist := s.Find(".release-list-title a").Attr("href")
		if !exist {
			return
		}
		links = append(links, link)
	})

	assert.EqualValues(t, []string{
		"/user2/repo1/releases/tag/draft-release",
		"/user2/repo1/releases/tag/v1.0",
		"/user2/repo1/releases/tag/v1.1",
	}, links)
}

func TestViewTagsList(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	link := repo.Link() + "/tags"

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", link)
	rsp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	tags := htmlDoc.Find(".tag-list-row-link")
	assert.Equal(t, 3, tags.Length())

	tagNames := make([]string, 0, 5)
	tags.Each(func(i int, s *goquery.Selection) {
		tagNames = append(tagNames, s.Text())
	})

	assert.EqualValues(t, []string{"v1.0", "delete-tag", "v1.1"}, tagNames)
}

func TestDownloadReleaseAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	tests.PrepareAttachmentsStorage(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	url := repo.Link() + "/releases/download/v1.1/README.md"

	req := NewRequest(t, "GET", url)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "GET", url)
	session := loginUser(t, "user2")
	session.MakeRequest(t, req, http.StatusOK)
}
