// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoIsEmpty(t *testing.T) {
	emptyRepo2Path := filepath.Join(testReposDir, "repo2_empty")
	repo, err := OpenRepository(t.Context(), emptyRepo2Path)
	assert.NoError(t, err)
	defer repo.Close()
	isEmpty, err := repo.IsEmpty()
	assert.NoError(t, err)
	assert.True(t, isEmpty)
}

// TestCloneNoFollowRedirects ensures the migration clone refuses HTTP redirects,
// so a remote cannot redirect to an otherwise-blocked address (SSRF). Without the
// option git follows the redirect and reaches the target.
func TestCloneNoFollowRedirects(t *testing.T) {
	var targetHit atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit.Store(true)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirect.Close()

	t.Run("FollowsRedirectByDefault", func(t *testing.T) {
		targetHit.Store(false)
		err := Clone(t.Context(), redirect.URL, filepath.Join(t.TempDir(), "dst"), CloneRepoOptions{})
		assert.Error(t, err)
		assert.True(t, targetHit.Load(), "git should reach the redirect target without the protection")
	})

	t.Run("RefusesRedirect", func(t *testing.T) {
		targetHit.Store(false)
		err := Clone(t.Context(), redirect.URL, filepath.Join(t.TempDir(), "dst"), CloneRepoOptions{NoFollowRedirects: true})
		assert.Error(t, err)
		assert.False(t, targetHit.Load(), "git must not follow the redirect to the target")
	})
}
