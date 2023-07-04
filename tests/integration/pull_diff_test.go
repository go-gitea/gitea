// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"
	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func TestPullDiff_CompletePRDiff(t *testing.T) {
	doTestPrDiff(t, "/user2/repo1/pulls/3/files", false, []string{"3", "iso-8859-1.txt"})
}

func TestPullDiff_SingleCommitPRDiff(t *testing.T) {
	doTestPrDiff(t, "/user2/repo1/pulls/3/commits/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd", true, []string{"iso-8859-1.txt"})
}

func TestPullDiff_CommitRangePRDiff(t *testing.T) {
	doTestPrDiff(t, "/user2/repo1/pulls/3/files/4a357436d925b5c974181ff12a994538ddc5a269..5f22f7d0d95d614d25a5b68592adb345a4b5c7fd", true, []string{"iso-8859-1.txt"})
}

func TestPullDiff_StartingFromCommitPRDiffFirstCommit(t *testing.T) {
	doTestPrDiff(t, "/user2/repo1/pulls/3/files/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd", true, []string{"3", "iso-8859-1.txt"})
}

func TestPullDiff_StartingFromCommitPRDiffLastCommit(t *testing.T) {
	doTestPrDiff(t, "/user2/repo1/pulls/3/files/4a357436d925b5c974181ff12a994538ddc5a269", true, []string{"3"})
}

func doTestPrDiff(t *testing.T, prDiffURL string, reviewBtnDisabled bool, expectedFilenames []string) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")

	req := NewRequest(t, "GET", "/pulls")
	session.MakeRequest(t, req, http.StatusOK)

	// Get all commits
	req = NewRequest(t, "GET", "/user2/repo1/pulls/3/commits")
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	// Ensure we got 2 commits in this PR
	commits := doc.doc.Find("#commits-table tbody tr td.sha a")
	assert.Equal(t, 2, commits.Length())

	// Get the given PR diff url
	req = NewRequest(t, "GET", prDiffURL)
	resp = session.MakeRequest(t, req, http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)

	// Assert all files are visible.
	fileContents := doc.doc.Find("div.file-content")
	numberOfFiles := fileContents.Length()

	assert.Equal(t, len(expectedFilenames), numberOfFiles)

	fileContents.Each(func(i int, s *goquery.Selection) {
		filename, _ := s.Attr("data-old-filename")
		assert.Equal(t, expectedFilenames[i], filename)
	})

	// Ensure the review button is enabled for full PR reviews
	assert.Equal(t, reviewBtnDisabled, doc.doc.Find(".js-btn-review").HasClass("disabled"))
}
