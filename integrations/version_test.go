// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	prepareTestEnv(t)

	setting.AppVer = "1.1.0+dev"
	req, err := http.NewRequest("GET", "/api/v1/version", nil)
	assert.NoError(t, err)
	resp := MakeRequest(req)

	var version gitea.ServerVersion
	decoder := json.NewDecoder(bytes.NewBuffer(resp.Body))
	assert.NoError(t, decoder.Decode(&version))

	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	assert.Equal(t, setting.AppVer, string(version.Version))
}
