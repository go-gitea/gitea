// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitedReader(t *testing.T) {
	data, err := io.ReadAll(NewLimitedReader(strings.NewReader("abc"), 3))
	assert.NoError(t, err)
	assert.Equal(t, "abc", string(data))

	r := NewLimitedReader(strings.NewReader("abcd"), 3)
	data, err = io.ReadAll(r)
	assert.ErrorIs(t, err, ErrContentTooLarge)
	assert.Equal(t, "abc", string(data))
	n, err := r.Read(make([]byte, 8))
	assert.Zero(t, n)
	assert.ErrorIs(t, err, ErrContentTooLarge)

	assert.Equal(t, NewLimitedReader(nil, 512*1024*1024), NewArchiveLimitedReader(nil, 1))
	assert.Equal(t, NewLimitedReader(nil, 1000*1024*1024), NewArchiveLimitedReader(nil, 10*1024*1024))
}
