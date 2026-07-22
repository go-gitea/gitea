// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	u, _ := url.Parse("file:///test")
	c := newClient(u, nil)
	assert.IsType(t, &FilesystemClient{}, c)

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
