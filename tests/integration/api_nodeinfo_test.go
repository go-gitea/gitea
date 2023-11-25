// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers"

	"github.com/stretchr/testify/assert"
)

func TestNodeinfo(t *testing.T) {
	setting.Federation.Enabled = true
	c = routers.NormalRoutes(context.TODO())
	defer func() {
		setting.Federation.Enabled = false
		c = routers.NormalRoutes(context.TODO())
	}()

	onGiteaRun(t, func(*testing.T, *url.URL) {
		req := NewRequestf(t, "GET", "/api/v1/nodeinfo")
		resp := MakeRequest(t, req, http.StatusOK)
		VerifyJSONSchema(t, resp, "nodeinfo_2.1.json")

		var nodeinfo api.NodeInfo
		DecodeJSON(t, resp, &nodeinfo)
		assert.True(t, nodeinfo.OpenRegistrations)
		assert.Equal(t, "gitea", nodeinfo.Software.Name)
		assert.Equal(t, 25, nodeinfo.Usage.Users.Total)
		assert.Equal(t, 18, nodeinfo.Usage.LocalPosts)
		assert.Equal(t, 3, nodeinfo.Usage.LocalComments)
	})
}
