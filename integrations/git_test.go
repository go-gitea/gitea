// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"net/http"
	"testing"

	"time"

	"github.com/stretchr/testify/assert"
	git "gopkg.in/src-d/go-git.v4"
)

func TestClonePush_ViaHTTP_NoLogin(t *testing.T) {
	prepareTestEnv(t)

	s := http.Server{
		Addr:    ":3000",
		Handler: mac,
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.ListenAndServe()

	r := git.NewMemoryRepository()
	err := r.Clone(&git.CloneOptions{URL: "http://localhost:3000/user2/repo1.git"})
	assert.NoError(t, err)

	empty, err := r.IsEmpty()
	assert.NoError(t, err)
	assert.Equal(t, false, empty)
}
