// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package filebuffer

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileBackedBuffer(t *testing.T) {
	for _, content := range []string{"test", "testtest"} {
		buf := New(5, t.TempDir())
		_, err := io.Copy(buf, strings.NewReader(content))
		assert.NoError(t, err)

		assert.EqualValues(t, len(content), buf.Size())

		data, err := io.ReadAll(buf)
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))

		assert.NoError(t, buf.Close())
	}
}
