// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models" // https://discourse.gitea.io/t/testfixtures-could-not-clean-table-access-no-such-table-access/4137/4

	"github.com/stretchr/testify/assert"
)

func TestActivityPubSignedPost(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	pubID := "https://example.com/pubID"
	c, err := NewClient(user, pubID)
	assert.NoError(t, err)

	expected := "BODY"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Regexp(t, regexp.MustCompile("^"+setting.Federation.DigestAlgorithm), r.Header.Get("Digest"))
		assert.Contains(t, r.Header.Get("Signature"), pubID)
		assert.Equal(t, r.Header.Get("Content-Type"), ActivityStreamsContentType)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, expected, string(body))
		fmt.Fprintf(w, expected)
	}))
	defer srv.Close()

	r, err := c.Post([]byte(expected), srv.URL)
	assert.NoError(t, err)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(body))
}
