// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"crypto/md5"
	"fmt"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func Test_artifactRoutes_buildArtifactURL(t *testing.T) {
	oldAppURL := setting.AppURL
	oldArtifactRootURL := setting.Actions.ArtifactRootURL
	defer func() {
		setting.AppURL = oldAppURL
		setting.Actions.ArtifactRootURL = oldArtifactRootURL
	}()

	routes := artifactRoutes{
		prefix: "/api/actions_pipeline",
	}

	{
		setting.AppURL = "https://gitea.com"
		setting.Actions.ArtifactRootURL = ""
		u := routes.buildArtifactURL(100, fmt.Sprintf("%x", md5.Sum([]byte("test"))), "download_url")
		assert.Equal(t, "https://gitea.com/api/actions_pipeline/_apis/pipelines/workflows/100/artifacts/098f6bcd4621d373cade4e832627b4f6/download_url", u)
	}

	{
		setting.AppURL = "https://gitea.com"
		setting.Actions.ArtifactRootURL = "http://gitea:3000"
		u := routes.buildArtifactURL(100, fmt.Sprintf("%x", md5.Sum([]byte("test"))), "download_url")
		assert.Equal(t, "http://gitea:3000/api/actions_pipeline/_apis/pipelines/workflows/100/artifacts/098f6bcd4621d373cade4e832627b4f6/download_url", u)
	}
}
