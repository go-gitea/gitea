// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/git"

	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
)

func onGiteaWebRun(t *testing.T, callback func(*testing.T, string)) {
	s := http.Server{
		Handler: mac,
	}

	listener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.Serve(listener)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	assert.NoError(t, err)

	callback(t, fmt.Sprintf("http://localhost:%s/", port))
}

func TestClone_ViaHTTP_NoLogin(t *testing.T) {
	prepareTestEnv(t)

	onGiteaWebRun(t, func(t *testing.T, urlPrefix string) {
		dstPath, err := ioutil.TempDir("", "repo1")
		assert.NoError(t, err)
		defer os.RemoveAll(dstPath)

		err = git.Clone(fmt.Sprintf("%suser2/repo1.git", urlPrefix),
			dstPath, git.CloneRepoOptions{})
		assert.NoError(t, err)

		assert.True(t, com.IsExist(filepath.Join(dstPath, "README.md")))
	})
}
