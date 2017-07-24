// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testRepoMigrate(t testing.TB, session *TestSession, cloneAddr, repoName string) *TestResponse {
	req := NewRequest(t, "GET", "/repo/migrate")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")

	uid, exists := htmlDoc.doc.Find("#uid").Attr("value")
	assert.True(t, exists, "The template has changed")

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"clone_addr": cloneAddr,
		"uid":        uid,
		"repo_name":  repoName,
	},
	)
	resp = session.MakeRequest(t, req, http.StatusFound)

	return resp
}

func TestRepoMigrate(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	testRepoMigrate(t, session, "https://github.com/go-gitea/git.git", "git")
}

func BenchmarkRepoMigrate(b *testing.B) {
	samples := []struct {
		url  string
		name string
	}{
		{url: "https://github.com/go-gitea/gitea.git", name: "gitea"},
		{url: "https://github.com/ethantkoenig/manyfiles.git", name: "manyfiles"},
		{url: "https://github.com/moby/moby.git", name: "moby"},
		{url: "https://github.com/golang/go.git", name: "go"},
		{url: "https://github.com/torvalds/linux.git", name: "linux"},
	}

	prepareTestEnv(b)
	session := loginUser(b, "user2")
	b.ResetTimer()

	for _, s := range samples {
		b.Run(s.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				testRepoMigrate(b, session, s.url, s.name)
			}

		})
	}
}
