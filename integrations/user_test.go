// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestViewUser(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2")
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}
