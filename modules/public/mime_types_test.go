// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsWellKnownSafeInlineMimeType(t *testing.T) {
	table := []struct {
		mimeType string
		safe     bool
	}{
		{
			mimeType: "text/plain; charset=utf-8",
			safe:     true,
		},
		{
			mimeType: "text/plain",
			safe:     true,
		},
		{
			mimeType: "application/pdf",
			safe:     true,
		},
		{
			mimeType: "application/zip",
			safe:     false,
		},
		{
			mimeType: "application/wasm",
			safe:     false,
		},
		{
			mimeType: "image/webp",
			safe:     true,
		},
		{
			mimeType: "IMAGE/Webp",
			safe:     true,
		},
		{
			mimeType: "text/javascript",
			safe:     false,
		},
		{
			mimeType: "text/javascript; charset=utf-8",
			safe:     false,
		},
	}

	for _, entry := range table {
		t.Run(entry.mimeType, func(t *testing.T) {
			assert.Equal(t, entry.safe, IsWellKnownSafeInlineMimeType(entry.mimeType))
		})
	}
}

func TestDetectWellKnownSafeInlineMimeType(t *testing.T) {
	table := []struct {
		ext      string
		mimeType string
		safe     bool
	}{
		{
			ext:      ".txt",
			mimeType: "text/plain; charset=utf-8",
			safe:     true,
		},
		{
			ext:      ".TxT",
			mimeType: "text/plain; charset=utf-8",
			safe:     true,
		},
		{
			ext:      ".pdf",
			mimeType: "application/pdf",
			safe:     true,
		},
		{
			ext:      ".wasm",
			mimeType: "application/wasm",
			safe:     false,
		},
		{
			ext:      ".webp",
			mimeType: "image/webp",
			safe:     true,
		},
		{
			ext:      ".js",
			mimeType: "text/javascript; charset=utf-8",
			safe:     false,
		},
		{
			ext:      ".mjs",
			mimeType: "text/javascript; charset=utf-8",
			safe:     false,
		},
		{
			ext:      ".MJS",
			mimeType: "text/javascript; charset=utf-8",
			safe:     false,
		},
	}

	for _, entry := range table {
		t.Run(entry.ext, func(t *testing.T) {
			mimeType, safe := DetectWellKnownSafeInlineMimeType(entry.ext)
			assert.Equal(t, entry.mimeType, mimeType)
			assert.Equal(t, entry.safe, safe)
		})
	}
}
