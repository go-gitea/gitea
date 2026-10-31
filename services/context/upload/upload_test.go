// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package upload

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {
	testContent := []byte(`This is a plain text file.`)
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(testContent)
	w.Close()

	kases := []struct {
		data         []byte
		fileName     string
		allowedTypes []string
		err          error
	}{
		{testContent, "test.txt", []string{""}, nil},
		{testContent, "dir/test.txt", []string{""}, nil},
		{testContent, "../../../test.txt", []string{""}, nil},
		{testContent, "test.txt", []string{"", ",", "|", "*/*", "*/*,", "*/*|", "text/plain"}, nil},
		{testContent, "dir/test.txt", []string{"text/plain"}, nil},
		{testContent, "/dir.txt/test.js", []string{".js"}, nil},
		{testContent, "test.txt", []string{" text/plain ", ".txt", " .txt,.js", " .txt|.js"}, nil},
		{testContent, "../../test.txt", []string{" .txt|.js"}, nil},
		{testContent, "test.txt", []string{" .txt ,.js ", "text/plain, .txt", "text/*", "text/*,.js"}, nil},
		{testContent, "test.txt", []string{"text/**", "application/x-gzip", ".zip", ".zip,.txtx", ".zip|.txtx"}, ErrFileTypeForbidden{"text/plain; charset=utf-8"}},
		{b.Bytes(), "test.txt", []string{"application/x-gzip"}, nil},
	}

	for _, kase := range kases {
		for _, allowedTypes := range kase.allowedTypes {
			assert.Equal(t, kase.err, Verify(kase.data, kase.fileName, allowedTypes))
		}
	}
}
