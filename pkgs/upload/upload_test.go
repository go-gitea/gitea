// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
		allowedTypes string
		err          error
	}{
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "dir/test.txt",
			allowedTypes: "",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "../../../test.txt",
			allowedTypes: "",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: ",",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "|",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "*/*",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "*/*,",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "*/*|",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "text/plain",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "dir/test.txt",
			allowedTypes: "text/plain",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "/dir.txt/test.js",
			allowedTypes: ".js",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: " text/plain ",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: ".txt",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: " .txt,.js",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: " .txt|.js",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "../../test.txt",
			allowedTypes: " .txt|.js",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: " .txt ,.js ",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "text/plain, .txt",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "text/*",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "text/*,.js",
			err:          nil,
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "text/**",
			err:          ErrFileTypeForbidden{"text/plain; charset=utf-8"},
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: "application/x-gzip",
			err:          ErrFileTypeForbidden{"text/plain; charset=utf-8"},
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: ".zip",
			err:          ErrFileTypeForbidden{"text/plain; charset=utf-8"},
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: ".zip,.txtx",
			err:          ErrFileTypeForbidden{"text/plain; charset=utf-8"},
		},
		{
			data:         testContent,
			fileName:     "test.txt",
			allowedTypes: ".zip|.txtx",
			err:          ErrFileTypeForbidden{"text/plain; charset=utf-8"},
		},
		{
			data:         b.Bytes(),
			fileName:     "test.txt",
			allowedTypes: "application/x-gzip",
			err:          nil,
		},
	}

	for _, kase := range kases {
		assert.Equal(t, kase.err, Verify(kase.data, kase.fileName, kase.allowedTypes))
	}
}
