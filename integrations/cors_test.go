// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func Test_CORSNotSet(t *testing.T) {
	setting.EnableCORS = false
	setting.NewContext()
	prepareTestEnv(t)
	req := NewRequestf(t, "GET", "/api/v1/version")
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	// body, _ := ioutil.ReadAll(resp.Body)
	corsHeader := reps.Header().Get("Access-Control-Allow-Origin")
	assert.Equal(t, corsHeader, "", "Access-Control-Allow-Origin: generated header should match") // header not set
}

func Test_CORSBasic(t *testing.T) {
	setting.EnableCORS = true
	setting.NewContext()

	prepareTestEnv(t)

	req := NewRequestf(t, "GET", "/api/v1/version")
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	corsHeader := reps.Header().Get("Access-Control-Allow-Origin")
	assert.Equal(t, corsHeader, "*", "Access-Control-Allow-Origin: generated header should match")

}