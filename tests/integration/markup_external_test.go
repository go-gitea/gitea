// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestExternalMarkupRenderer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	if !setting.Database.UseSQLite3 {
		t.Skip()
		return
	}

	const repoURL = "user30/renderer"
	req := NewRequest(t, "GET", repoURL+"/src/branch/master/README.html")
	resp := MakeRequest(t, req, http.StatusOK)
	bs, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	doc := NewHTMLParser(t, bytes.NewBuffer(bs))
	div := doc.Find("div.file-view")
	assert.EqualValues(t, "<div>	test external renderer</div>", strings.TrimSpace(div.Text()))
}
