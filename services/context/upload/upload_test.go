// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package upload

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {
	text := []byte(`This is a plain text file.`)
	gzip := []byte("\x1F\x8B\x08\x00\x00\x00\x00\x00\x00\xFF")
	jxl := []byte("\xFF\x0A....")
	cases := []struct {
		data         []byte
		fileName     string
		allowedTypes []string
		err          error
	}{
		{text, "test.txt", []string{""}, nil},
		{text, "dir/test.txt", []string{""}, nil},
		{text, "../../../test.txt", []string{""}, nil},
		{text, "test.txt", []string{"", ",", "|", "*/*", "*/*,", "*/*|", "text/plain"}, nil},
		{text, "dir/test.txt", []string{"text/plain"}, nil},
		{text, "/dir.txt/test.js", []string{".js"}, nil},
		{text, "test.txt", []string{" text/plain ", ".txt", " .txt,.js", " .txt|.js"}, nil},
		{text, "../../test.txt", []string{" .txt|.js"}, nil},
		{text, "test.txt", []string{" .txt ,.js ", "text/plain, .txt", "text/*", "text/*,.js"}, nil},
		{text, "test.txt", []string{"text/**", "application/x-gzip", ".zip", ".zip,.txtx", ".zip|.txtx"}, ErrFileTypeForbidden{"text/plain"}},
		{gzip, "test.txt", []string{"application/x-gzip"}, nil},
		{jxl, "test.jxl", []string{"image/jxl"}, nil},
	}

	for _, tc := range cases {
		for _, allowedTypes := range tc.allowedTypes {
			assert.Equal(t, tc.err, Verify(tc.data, tc.fileName, allowedTypes))
		}
	}
}
