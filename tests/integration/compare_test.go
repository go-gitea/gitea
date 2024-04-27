// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestCompareTag(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/v1.1...master")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	selection := htmlDoc.doc.Find(".choose.branch .filter.dropdown")
	// A dropdown for both base and head.
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")

	req = NewRequest(t, "GET", "/user2/repo1/compare/invalid")
	resp = session.MakeRequest(t, req, http.StatusNotFound)
	assert.False(t, strings.Contains(resp.Body.String(), "/assets/img/500.png"), "expect 404 page not 500")
}

// Compare with inferred default branch (master)
func TestCompareDefault(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/v1.1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	selection := htmlDoc.doc.Find(".choose.branch .filter.dropdown")
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")
}

// Ensure the comparison matches what we expect
func inspectCompare(t *testing.T, htmlDoc *HTMLDoc, diffCount int, diffChanges []string) {
	selection := htmlDoc.doc.Find("#diff-file-boxes").Children()

	assert.Lenf(t, selection.Nodes, diffCount, "Expected %v diffed files, found: %v", diffCount, len(selection.Nodes))

	for _, diffChange := range diffChanges {
		selection = htmlDoc.doc.Find(fmt.Sprintf("[data-new-filename=\"%s\"]", diffChange))
		assert.Lenf(t, selection.Nodes, 1, "Expected 1 match for [data-new-filename=\"%s\"], found: %v", diffChange, len(selection.Nodes))
	}
}

// Git commit graph for repo20
// * 8babce9 (origin/remove-files-b) Add a dummy file
// * b67e43a Delete test.csv and link_hi
// | * cfe3b3c (origin/remove-files-a) Delete test.csv and link_hi
// |/
// * c8e31bc (origin/add-csv) Add test csv file
// * 808038d (HEAD -> master, origin/master, origin/HEAD) Added test links

func TestCompareBranches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Indirect compare remove-files-b (head) with add-csv (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added
	req := NewRequest(t, "GET", "/user2/repo20/compare/add-csv...remove-files-b")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	diffCount := 3
	diffChanges := []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Indirect compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-a...remove-files-b")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 3
	diffChanges = []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Indirect compare remove-files-a (head) with remove-files-b (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b...remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 2
	diffChanges = []string{"link_hi", "test.csv"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Direct compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'test.txt' is deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b..remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 1
	diffChanges = []string{"test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)
}
