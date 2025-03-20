// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRenderPanicErrorPage(t *testing.T) {
	w := httptest.NewRecorder()
	req := &http.Request{URL: &url.URL{}}
	req = req.WithContext(reqctx.NewRequestContextForTest(t.Context()))
	RenderPanicErrorPage(w, req, errors.New("fake panic error (for test only)"))
	respContent := w.Body.String()
	assert.Contains(t, respContent, `class="page-content status-page-500"`)
	assert.Contains(t, respContent, `</html>`)
	assert.Contains(t, respContent, `lang="en-US"`) // make sure the locale work

	// the 500 page doesn't have normal pages footer, it makes it easier to distinguish a normal page and a failed page.
	// especially when a sub-template causes page error, the HTTP response code is still 200,
	// the different "footer" is the only way to know whether a page is fully rendered without error.
	assert.False(t, test.IsNormalPageCompleted(respContent))
}

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
