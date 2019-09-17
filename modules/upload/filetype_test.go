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
		allowedTypes []string
		err          error
	}{
		{
			data:         testContent,
			allowedTypes: []string{"text/plain"},
			err:          nil,
		},
		{
			data:         testContent,
			allowedTypes: []string{"application/x-gzip"},
			err:          ErrFileTypeForbidden{"text/plain; charset=utf-8"},
		},
		{
			data:         b.Bytes(),
			allowedTypes: []string{"application/x-gzip"},
			err:          nil,
		},
	}

	for _, kase := range kases {
		assert.Equal(t, kase.err, VerifyAllowedContentType(kase.data, kase.allowedTypes))
	}
}
