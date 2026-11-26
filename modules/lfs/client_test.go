// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	u := endpointFromLocalPath(t.TempDir())
	c := newClient(u, nil)
	require.IsType(t, &FilesystemClient{}, c)
	localPath, err := c.(*FilesystemClient).objectPath("invalid") //nolint:forcetypeassert // type is expected
	assert.Empty(t, localPath)
	assert.ErrorIs(t, err, ErrInvalidOIDFormat)

	missing := []Pointer{{Oid: strings.Repeat("0", 64)}, {Oid: strings.Repeat("1", 64)}}
	var received []Pointer
	err = c.Download(t.Context(), "", missing, func(pointer Pointer, reader io.ReadCloser, objectErr error) error {
		received = append(received, pointer)
		assert.Nil(t, reader)
		assert.ErrorIs(t, objectErr, ErrObjectNotExist)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, missing, received)

	u, _ = url.Parse("https://test.com/lfs")
	c = newClient(u, nil)
	assert.IsType(t, &HTTPClient{}, c)
}

func TestNewClientFromEndpoint(t *testing.T) {
	client, err := NewClientFromEndpoint("ssh://git@example.com/owner/repo.git", "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	client, err = NewClientFromEndpoint("ftp://example.com/owner/repo.git", "", nil)
	assert.Nil(t, client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to determine LFS endpoint")
}
