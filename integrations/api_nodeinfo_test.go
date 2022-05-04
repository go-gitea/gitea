// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestNodeinfo(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		setting.Federation.Enabled = true
		defer func() {
			setting.Federation.Enabled = false
		}()

		req := NewRequestf(t, "GET", "/api/v1/nodeinfo")
		resp := MakeRequest(t, req, http.StatusOK)
		var nodeinfo api.NodeInfo
		DecodeJSON(t, resp, &nodeinfo)
		assert.True(t, nodeinfo.OpenRegistrations)
		assert.Equal(t, "gitea", nodeinfo.Software.Name)
		assert.Equal(t, 23, nodeinfo.Usage.Users.Total)
		assert.Equal(t, 15, nodeinfo.Usage.LocalPosts)
		assert.Equal(t, 2, nodeinfo.Usage.LocalComments)
	})
}
