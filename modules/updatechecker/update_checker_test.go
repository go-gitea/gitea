// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/system"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memoryStateStore records the latest version passed to Set without a database.
type memoryStateStore struct {
	versions []string
}

func (s *memoryStateStore) Get(ctx context.Context, item system.StateItem) error {
	return fmt.Errorf("not implemented")
}

func (s *memoryStateStore) Set(ctx context.Context, item system.StateItem) error {
	state, ok := item.(*CheckerState)
	if !ok {
		return fmt.Errorf("unexpected state item: %T", item)
	}
	s.versions = append(s.versions, state.LatestVersion)
	return nil
}

func TestGiteaUpdateCheckerResponseSizeLimit(t *testing.T) {
	store := &memoryStateStore{}
	oldAppState := system.AppState
	system.AppState = store
	defer func() { system.AppState = oldAppState }()

	// A normal version-check response is only a few dozen bytes.
	small := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"latest":{"version":"1.25.0"}}`))
	}))
	defer small.Close()

	err := GiteaUpdateChecker(small.URL)
	require.NoError(t, err)
	require.Len(t, store.versions, 1)
	assert.Equal(t, "1.25.0", store.versions[0])

	// A response larger than the limit must fail instead of being buffered in full.
	// The body is valid JSON so that only the size limit can reject it.
	oversized := `{"latest":{"version":"` + strings.Repeat("9", maxUpdateCheckerResponseSize) + `"}}`
	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(oversized))
	}))
	defer large.Close()

	err = GiteaUpdateChecker(large.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
	assert.Len(t, store.versions, 1, "oversized response must not update the stored version")
}
