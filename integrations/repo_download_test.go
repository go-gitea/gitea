// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// based on repo_commits_test.go

package integrations

import (
	"encoding/base64"
	"net/http"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

// gitea/integrations/integration_test.go -> NewRequest

// gitea/routers/web/repo/repo.go: addCommitObjectResponseHeader
func TestRepoDownloadWithCommitObject(t *testing.T) {
	defer prepareTestEnv(t)()

	// Request repository commits page
	req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp := MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	// Get first commit URL
	commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Attr("href")
	assert.True(t, exists)
	assert.NotEmpty(t, commitURL)

	commitHash := path.Base(commitURL)

	q2 := NewRequest(t, "GET", "/user2/repo1/archive/"+commitHash+".tar.gz")
	assert.NotNil(t, q2)
	q2.Header.Set("X-Commit-Object", "1") // request the commit object
	r2 := MakeRequest(t, q2, http.StatusOK)
	// decode the commit object
	str64 := r2.Header().Get("X-Commit-Object")
	bytes, err := base64.StdEncoding.DecodeString(str64)
	assert.NoError(t, err)
	str := string(bytes)
	strExpected := ("" +
		"tree 2a2f1d4670728a2e10049e345bd7a276468beab6\n" +
		"author user1 <address1@example.com> 1489956479 -0400\n" +
		"committer Ethan Koenig <ethantkoenig@gmail.com> 1489956479 -0400\n" +
		"\n" +
		"Initial commit\n")
	assert.Equal(t, str, strExpected)
}
