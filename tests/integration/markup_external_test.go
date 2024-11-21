// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/external"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestExternalMarkupRenderer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	if !setting.Database.Type.IsSQLite3() {
		t.Skip()
		return
	}

	req := NewRequest(t, "GET", "/user30/renderer/src/branch/master/README.html")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header().Get("Content-Type"))

	bs, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	doc := NewHTMLParser(t, bytes.NewBuffer(bs))
	div := doc.Find("div.file-view")
	data, err := div.Html()
	assert.NoError(t, err)
	assert.EqualValues(t, "<div>\n\ttest external renderer\n</div>", strings.TrimSpace(data))

	r := markup.GetRendererByFileName("a.html").(*external.Renderer)
	r.RenderContentMode = setting.RenderContentModeIframe

	req = NewRequest(t, "GET", "/user30/renderer/src/branch/master/README.html")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header().Get("Content-Type"))
	bs, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	doc = NewHTMLParser(t, bytes.NewBuffer(bs))
	iframe := doc.Find("iframe")
	assert.EqualValues(t, "/user30/renderer/render/branch/master/README.html", iframe.AttrOr("src", ""))

	req = NewRequest(t, "GET", "/user30/renderer/render/branch/master/README.html")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, "text/html; charset=utf-8", resp.Header().Get("Content-Type"))
	bs, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.EqualValues(t, "frame-src 'self'; sandbox allow-scripts", resp.Header().Get("Content-Security-Policy"))
	assert.EqualValues(t, "<div>\n\ttest external renderer\n</div>\n", string(bs))
}
