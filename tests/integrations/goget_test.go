// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGoGet(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/blah/glah/plah?go-get=1")
	resp := MakeRequest(t, req, http.StatusOK)

	expected := fmt.Sprintf(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="%[1]s:%[2]s/blah/glah git %[3]sblah/glah.git">
		<meta name="go-source" content="%[1]s:%[2]s/blah/glah _ %[3]sblah/glah/src/branch/master{/dir} %[3]sblah/glah/src/branch/master{/dir}/{file}#L{line}">
	</head>
	<body>
		go get --insecure %[1]s:%[2]s/blah/glah
	</body>
</html>`, setting.Domain, setting.HTTPPort, setting.AppURL)

	assert.Equal(t, expected, resp.Body.String())
}
