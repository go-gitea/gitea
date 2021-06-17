// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"testing"

	"code.gitea.io/gitea/modules/generate"
	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

func TestLFSRootURL(t *testing.T) {
	AppURL = "http://localhost:3000"
	LFS.RootURL = ""

	rootURL := GetLFSRootURL()
	assert.Equal(t, rootURL, AppURL)

	LFS.RootURL = "http://localhost:3001"
	rootURL = GetLFSRootURL()
	assert.Equal(t, rootURL, LFS.RootURL)
}

func TestLFSRootURLFormatNoTrailingSlash(t *testing.T) {
	Cfg = ini.Empty()

	serverSec, err := Cfg.NewSection("server")
	assert.Equal(t, err, nil)
	_, err = serverSec.NewKey("LFS_START_SERVER", "true")
	assert.Equal(t, err, nil)
	_, err = serverSec.NewKey("LFS_ROOT_URL", "http://localhost:3001")
	assert.Equal(t, err, nil)
	LFS.JWTSecretBase64, err = generate.NewJwtSecret()
	assert.Equal(t, err, nil)

	newLFSService()

	rootURL := GetLFSRootURL()
	assert.Equal(t, rootURL, "http://localhost:3001/")
}

func TestLFSRootURLFormatWithTrailingSlash(t *testing.T) {
	Cfg = ini.Empty()

	serverSec, err := Cfg.NewSection("server")
	assert.Equal(t, err, nil)
	_, err = serverSec.NewKey("LFS_START_SERVER", "true")
	assert.Equal(t, err, nil)
	_, err = serverSec.NewKey("LFS_ROOT_URL", "http://localhost:3001/")
	assert.Equal(t, err, nil)
	LFS.JWTSecretBase64, err = generate.NewJwtSecret()
	assert.Equal(t, err, nil)

	newLFSService()

	rootURL := GetLFSRootURL()
	assert.Equal(t, rootURL, "http://localhost:3001/")
}
