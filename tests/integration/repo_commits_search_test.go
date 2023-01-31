// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testRepoCommitsSearch(t *testing.T, query, commit string) {
	session := loginUser(t, "user2")

	// Request repository commits page
	req := NewRequestf(t, "GET", "/user2/commits_search_test/commits/branch/master/search?q=%s", url.QueryEscape(query))
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	sel := doc.doc.Find("#commits-table tbody tr td.sha a")
	assert.EqualValues(t, commit, strings.TrimSpace(sel.Text()))
}

func TestRepoCommitsSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	testRepoCommitsSearch(t, "e8eabd", "")
	testRepoCommitsSearch(t, "38a9cb", "")
	testRepoCommitsSearch(t, "6e8e", "6e8eabd9a7")
	testRepoCommitsSearch(t, "58e97", "58e97d1a24")
	testRepoCommitsSearch(t, "author:alice", "6e8eabd9a7")
	testRepoCommitsSearch(t, "author:alice 6e8ea", "6e8eabd9a7")
	testRepoCommitsSearch(t, "committer:Tom", "58e97d1a24")
	testRepoCommitsSearch(t, "author:bob commit-4", "58e97d1a24")
	testRepoCommitsSearch(t, "author:bob commit after:2019-03-03", "58e97d1a24")
	testRepoCommitsSearch(t, "committer:alice 6e8e before:2019-03-02", "6e8eabd9a7")
	testRepoCommitsSearch(t, "committer:alice commit before:2019-03-02", "6e8eabd9a7")
	testRepoCommitsSearch(t, "committer:alice author:tom commit before:2019-03-04 after:2019-03-02", "0a8499a22a")
}
