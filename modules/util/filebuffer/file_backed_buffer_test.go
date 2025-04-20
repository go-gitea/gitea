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
	cases := []struct {
		MaxMemorySize int
		Data          string
	}{
		{5, "test"},
		{5, "testtest"},
	}

	for _, c := range cases {
		buf := New(c.MaxMemorySize, t.TempDir())
		_, err := io.Copy(buf, strings.NewReader(c.Data))
		assert.NoError(t, err)

		assert.EqualValues(t, len(c.Data), buf.Size())

		data, err := io.ReadAll(buf)
		assert.NoError(t, err)
		assert.Equal(t, c.Data, string(data))

		assert.NoError(t, buf.Close())
	}
}
