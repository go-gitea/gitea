// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"context"
	"io"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilesystemClientRejectsInvalidPointer(t *testing.T) {
	u, _ := url.Parse("file:///data")
	c := newFilesystemClient(u)

	// A malformed OID must never be turned into a filesystem path: a short OID used
	// to slice oid[0:2]/oid[2:4] out of range, and a traversal OID escaped the objects dir.
	for _, p := range []Pointer{{Oid: "ab", Size: 1}, {Oid: "../../../../etc/hosts", Size: 1}} {
		assert.Error(t, c.Download(context.Background(), []Pointer{p},
			func(Pointer, io.ReadCloser, error) error { return nil }))
		assert.Error(t, c.Upload(context.Background(), []Pointer{p},
			func(Pointer, error) (io.ReadCloser, error) { return nil, nil }))
	}
}
