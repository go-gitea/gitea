// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"mime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentDisposition(t *testing.T) {
	type testEntry struct {
		disposition ContentDispositionType
		filename    string
		header      string
	}
	table := []testEntry{
		{disposition: ContentDispositionInline, filename: "test.txt", header: "inline; filename=test.txt"},
		{disposition: ContentDispositionInline, filename: "test❌.txt", header: "inline; filename=test_.txt; filename*=utf-8''test%E2%9D%8C.txt"},
		{disposition: ContentDispositionInline, filename: "test ❌.txt", header: "inline; filename=\"test _.txt\"; filename*=utf-8''test%20%E2%9D%8C.txt"},
		{disposition: ContentDispositionInline, filename: "\"test.txt", header: "inline; filename=\"\\\"test.txt\""},
		{disposition: ContentDispositionInline, filename: "hello\tworld.txt", header: "inline; filename=\"hello\tworld.txt\""},
		{disposition: ContentDispositionAttachment, filename: "hello\tworld.txt", header: "attachment; filename=\"hello\tworld.txt\""},
		{disposition: ContentDispositionAttachment, filename: "hello\nworld.txt", header: "attachment; filename=hello_world.txt; filename*=utf-8''hello%0Aworld.txt"},
		{disposition: ContentDispositionAttachment, filename: "hello\rworld.txt", header: "attachment; filename=hello_world.txt; filename*=utf-8''hello%0Dworld.txt"},
	}

	// Check the needsEncodingRune replacer ranges except tab that is checked above
	// Any change in behavior should fail here
	for c := ' '; !needsEncodingRune(c); c++ {
		var header string
		switch {
		case strings.ContainsAny(string(c), ` (),/:;<=>?@[]`):
			header = "inline; filename=\"hello" + string(c) + "world.txt\""
		case strings.ContainsAny(string(c), `"\`):
			// This document advises against for backslash in quoted form:
			// https://datatracker.ietf.org/doc/html/rfc6266#appendix-D
			// However the mime package is not generating the filename* in this scenario
			header = "inline; filename=\"hello\\" + string(c) + "world.txt\""
		default:
			header = "inline; filename=hello" + string(c) + "world.txt"
		}
		table = append(table, testEntry{
			disposition: ContentDispositionInline,
			filename:    "hello" + string(c) + "world.txt",
			header:      header,
		})
	}

	for _, entry := range table {
		t.Run(string(entry.disposition)+"_"+entry.filename, func(t *testing.T) {
			encoded := encodeContentDisposition(entry.disposition, entry.filename)
			assert.Equal(t, entry.header, encoded)
			disposition, params, err := mime.ParseMediaType(encoded)
			require.NoError(t, err)
			assert.Equal(t, string(entry.disposition), disposition)
			assert.Equal(t, entry.filename, params["filename"])
		})
	}
}
