// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestViewUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2")
	MakeRequest(t, req, http.StatusOK)
}

func TestRenameUsername(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"_csrf":    GetCSRF(t, session, "/user/settings"),
		"name":     "newUsername",
		"email":    "user2@example.com",
		"language": "en-US",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "newUsername"})
	unittest.AssertNotExistsBean(t, &user_model.User{Name: "user2"})
}

func TestRenameInvalidUsername(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	invalidUsernames := []string{
		"%2f*",
		"%2f.",
		"%2f..",
		"%00",
		"thisHas ASpace",
		"p<A>tho>lo<gical",
		".",
		"..",
		".well-known",
		".abc",
		"abc.",
		"a..bc",
		"a...bc",
		"a.-bc",
		"a._bc",
		"a_-bc",
		"a/bc",
		"☁️",
		"-",
		"--diff",
		"-im-here",
		"a space",
	}

	session := loginUser(t, "user2")
	for _, invalidUsername := range invalidUsernames {
		t.Logf("Testing username %s", invalidUsername)

		req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"_csrf": GetCSRF(t, session, "/user/settings"),
			"name":  invalidUsername,
			"email": "user2@example.com",
		})
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t,
			htmlDoc.doc.Find(".ui.negative.message").Text(),
			translation.NewLocale("en-US").Tr("form.username_error"),
		)

		unittest.AssertNotExistsBean(t, &user_model.User{Name: invalidUsername})
	}
}

func TestRenameReservedUsername(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	reservedUsernames := []string{
		// ".", "..", ".well-known", // The names are not only reserved but also invalid
		"admin",
		"api",
		"assets",
		"attachments",
		"avatar",
		"avatars",
		"captcha",
		"commits",
		"debug",
		"error",
		"explore",
		"favicon.ico",
		"ghost",
		"issues",
		"login",
		"manifest.json",
		"metrics",
		"milestones",
		"new",
		"notifications",
		"org",
		"pulls",
		"raw",
		"repo",
		"repo-avatars",
		"robots.txt",
		"search",
		"serviceworker.js",
		"ssh_info",
		"swagger.v1.json",
		"user",
		"v2",
	}

	session := loginUser(t, "user2")
	for _, reservedUsername := range reservedUsernames {
		t.Logf("Testing username %s", reservedUsername)
		req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"_csrf":    GetCSRF(t, session, "/user/settings"),
			"name":     reservedUsername,
			"email":    "user2@example.com",
			"language": "en-US",
		})
		resp := session.MakeRequest(t, req, http.StatusSeeOther)

		req = NewRequest(t, "GET", test.RedirectURL(resp))
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t,
			htmlDoc.doc.Find(".ui.negative.message").Text(),
			translation.NewLocale("en-US").Tr("user.form.name_reserved", reservedUsername),
		)

		unittest.AssertNotExistsBean(t, &user_model.User{Name: reservedUsername})
	}
}

func TestExportUserGPGKeys(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// Export empty key list
	testExportUserGPGKeys(t, "user1", `-----BEGIN PGP PUBLIC KEY BLOCK-----
Note: This user hasn't uploaded any GPG keys.


=twTO
-----END PGP PUBLIC KEY BLOCK-----
`)
	// Import key
	// User1 <user1@example.com>
	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteGPGKey)
	testCreateGPGKey(t, session.MakeRequest, token, http.StatusCreated, `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFyy/VUBCADJ7zbM20Z1RWmFoVgp5WkQfI2rU1Vj9cQHes9i42wVLLtcbPeo
QzubgzvMPITDy7nfWxgSf83E23DoHQ1ACFbQh/6eFSRrjsusp3YQ/08NSfPPbcu8
0M5G+VGwSfzS5uEcwBVQmHyKdcOZIERTNMtYZx1C3bjLD1XVJHvWz9D72Uq4qeO3
8SR+lzp5n6ppUakcmRnxt3nGRBj1+hEGkdgzyPo93iy+WioegY2lwCA9xMEo5dah
BmYxWx51zyiXYlReTaxlyb3/nuSUt8IcW3Q8zjdtJj4Nu8U1SpV8EdaA1I9IPbHW
510OSLmD3XhqHH5m6mIxL1YoWxk3V7gpDROtABEBAAG0GVVzZXIxIDx1c2VyMUBl
eGFtcGxlLmNvbT6JAU4EEwEIADgWIQTQEbrYxmXsp1z3j7z9+v0I6RSEHwUCXLL9
VQIbAwULCQgHAgYVCgkICwIEFgIDAQIeAQIXgAAKCRD9+v0I6RSEH22YCACFqL5+
6M0m18AMC/pumcpnnmvAS1GrrKTF8nOROA1augZwp1WCNuKw2R6uOJIHANrYECSn
u7+j6GBP2gbIW8mSAzS6HWCs7GGiPpVtT4wcu8wljUI6BxjpyZtoEkriyBjt6HfK
rkegbkuySoJvjq4IcO5D1LB1JWgsUjMYQJj/ZpBIzVtjG9QtFSOiT1Hct4PoZHdC
nsdSgyCkwRZXG+u3kT/wP9F663ba4o16vYlz3dCGo66lF2tyoG3qcyZ1OUzUrnuv
96ytAzT6XIhrE0nVoBprMxFF5zExotJD3bHjcGBFNLf944bhjKee3U6t9+OsfJVC
l7N5xxIawCuTQdbfuQENBFyy/VUBCADe61yGEoTwKfsOKIhxLaNoRmD883O0tiWt
soO/HPj9dPQLTOiwXgSgSCd8C+LNxGKct87wgFozpah4tDLC6c0nALuHJ0SLbkfz
55aRhLeOOcrAydatDp72GroXzqpZ0xZBk5wjIWdgEol2GmVRM8QGbeuakU/HVz5y
lPzxUUocgdbSi3GE3zbzijQzVJdyL/kw/KP7pKT/PPKKJ2C5NQDLy0XGKEHddXGR
EWKkVlRalxq/TjfaMR0bi3MpezBsQmp99ATPO/d7trayZUxQHRtXzGFiOXfDHATr
qN730sODjqvU+mpc/SHCRwh9qWDjZRHSuKU5YDBjb5jIQJivZsQ/ABEBAAGJATYE
GAEIACAWIQTQEbrYxmXsp1z3j7z9+v0I6RSEHwUCXLL9VQIbDAAKCRD9+v0I6RSE
H7WoB/4tXl+97rQ6owPCGSVp1Xbwt2521V7COgsOFRVTRTryEWxRW8mm0S7wQvax
C0TLXKur6NVYQMn01iyL+FZzRpEWNuYF3f9QeeLJ/+l2DafESNhNTy17+RPmacK6
21dccpqchByVw/UMDeHSyjQLiG2lxzt8Gfx2gHmSbrq3aWovTGyz6JTffZvfy/n2
0Hm437OBPazO0gZyXhdV2PE5RSUfvAgm44235tcV5EV0d32TJDfv61+Vr2GUbah6
7XhJ1v6JYuh8kaYaEz8OpZDeh7f6Ho6PzJrsy/TKTKhGgZNINj1iaPFyOkQgKR5M
GrE0MHOxUbc9tbtyk0F1SuzREUBH
=DDXw
-----END PGP PUBLIC KEY BLOCK-----
`)
	// Export new key
	testExportUserGPGKeys(t, "user1", `-----BEGIN PGP PUBLIC KEY BLOCK-----

xsBNBFyy/VUBCADJ7zbM20Z1RWmFoVgp5WkQfI2rU1Vj9cQHes9i42wVLLtcbPeo
QzubgzvMPITDy7nfWxgSf83E23DoHQ1ACFbQh/6eFSRrjsusp3YQ/08NSfPPbcu8
0M5G+VGwSfzS5uEcwBVQmHyKdcOZIERTNMtYZx1C3bjLD1XVJHvWz9D72Uq4qeO3
8SR+lzp5n6ppUakcmRnxt3nGRBj1+hEGkdgzyPo93iy+WioegY2lwCA9xMEo5dah
BmYxWx51zyiXYlReTaxlyb3/nuSUt8IcW3Q8zjdtJj4Nu8U1SpV8EdaA1I9IPbHW
510OSLmD3XhqHH5m6mIxL1YoWxk3V7gpDROtABEBAAHNGVVzZXIxIDx1c2VyMUBl
eGFtcGxlLmNvbT7CwI4EEwEIADgWIQTQEbrYxmXsp1z3j7z9+v0I6RSEHwUCXLL9
VQIbAwULCQgHAgYVCgkICwIEFgIDAQIeAQIXgAAKCRD9+v0I6RSEH22YCACFqL5+
6M0m18AMC/pumcpnnmvAS1GrrKTF8nOROA1augZwp1WCNuKw2R6uOJIHANrYECSn
u7+j6GBP2gbIW8mSAzS6HWCs7GGiPpVtT4wcu8wljUI6BxjpyZtoEkriyBjt6HfK
rkegbkuySoJvjq4IcO5D1LB1JWgsUjMYQJj/ZpBIzVtjG9QtFSOiT1Hct4PoZHdC
nsdSgyCkwRZXG+u3kT/wP9F663ba4o16vYlz3dCGo66lF2tyoG3qcyZ1OUzUrnuv
96ytAzT6XIhrE0nVoBprMxFF5zExotJD3bHjcGBFNLf944bhjKee3U6t9+OsfJVC
l7N5xxIawCuTQdbfzsBNBFyy/VUBCADe61yGEoTwKfsOKIhxLaNoRmD883O0tiWt
soO/HPj9dPQLTOiwXgSgSCd8C+LNxGKct87wgFozpah4tDLC6c0nALuHJ0SLbkfz
55aRhLeOOcrAydatDp72GroXzqpZ0xZBk5wjIWdgEol2GmVRM8QGbeuakU/HVz5y
lPzxUUocgdbSi3GE3zbzijQzVJdyL/kw/KP7pKT/PPKKJ2C5NQDLy0XGKEHddXGR
EWKkVlRalxq/TjfaMR0bi3MpezBsQmp99ATPO/d7trayZUxQHRtXzGFiOXfDHATr
qN730sODjqvU+mpc/SHCRwh9qWDjZRHSuKU5YDBjb5jIQJivZsQ/ABEBAAHCwHYE
GAEIACAWIQTQEbrYxmXsp1z3j7z9+v0I6RSEHwUCXLL9VQIbDAAKCRD9+v0I6RSE
H7WoB/4tXl+97rQ6owPCGSVp1Xbwt2521V7COgsOFRVTRTryEWxRW8mm0S7wQvax
C0TLXKur6NVYQMn01iyL+FZzRpEWNuYF3f9QeeLJ/+l2DafESNhNTy17+RPmacK6
21dccpqchByVw/UMDeHSyjQLiG2lxzt8Gfx2gHmSbrq3aWovTGyz6JTffZvfy/n2
0Hm437OBPazO0gZyXhdV2PE5RSUfvAgm44235tcV5EV0d32TJDfv61+Vr2GUbah6
7XhJ1v6JYuh8kaYaEz8OpZDeh7f6Ho6PzJrsy/TKTKhGgZNINj1iaPFyOkQgKR5M
GrE0MHOxUbc9tbtyk0F1SuzREUBH
=WFf5
-----END PGP PUBLIC KEY BLOCK-----
`)
}

func testExportUserGPGKeys(t *testing.T, user, expected string) {
	session := loginUser(t, user)
	t.Logf("Testing username %s export gpg keys", user)
	req := NewRequest(t, "GET", "/"+user+".gpg")
	resp := session.MakeRequest(t, req, http.StatusOK)
	// t.Log(resp.Body.String())
	assert.Equal(t, expected, resp.Body.String())
}

func TestGetUserRss(t *testing.T) {
	user34 := "the_34-user.with.all.allowedChars"
	req := NewRequestf(t, "GET", "/%s.rss", user34)
	resp := MakeRequest(t, req, http.StatusOK)
	if assert.EqualValues(t, "application/rss+xml;charset=utf-8", resp.Header().Get("Content-Type")) {
		rssDoc := NewHTMLParser(t, resp.Body).Find("channel")
		title, _ := rssDoc.ChildrenFiltered("title").Html()
		assert.EqualValues(t, "Feed of &#34;the_1-user.with.all.allowedChars&#34;", title)
		description, _ := rssDoc.ChildrenFiltered("description").Html()
		assert.EqualValues(t, "&lt;p&gt;some &lt;a href=&#34;https://commonmark.org/&#34; rel=&#34;nofollow&#34;&gt;commonmark&lt;/a&gt;!&lt;/p&gt;\n", description)
	}
}

func TestListStopWatches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	req := NewRequestf(t, "GET", "/user/stopwatches")
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiWatches []*api.StopWatch
	DecodeJSON(t, resp, &apiWatches)
	stopwatch := unittest.AssertExistsAndLoadBean(t, &issues_model.Stopwatch{UserID: owner.ID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: stopwatch.IssueID})
	if assert.Len(t, apiWatches, 1) {
		assert.EqualValues(t, stopwatch.CreatedUnix.AsTime().Unix(), apiWatches[0].Created.Unix())
		assert.EqualValues(t, issue.Index, apiWatches[0].IssueIndex)
		assert.EqualValues(t, issue.Title, apiWatches[0].IssueTitle)
		assert.EqualValues(t, repo.Name, apiWatches[0].RepoName)
		assert.EqualValues(t, repo.OwnerName, apiWatches[0].RepoOwnerName)
		assert.Greater(t, apiWatches[0].Seconds, int64(0))
	}
}
