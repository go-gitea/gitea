// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorTranslatable(t *testing.T) {
	var err error

	err = ErrorWrapTranslatable(io.EOF, "key", 1)
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, "EOF", err.Error())
	wrapped, ok := err.(*errorTranslatableWrapper)
	require.True(t, ok)
	assert.Equal(t, "key", wrapped.trKey)
	assert.Equal(t, []any{1}, wrapped.trArgs)

	err = ErrorWrap(err, "new msg %d", 100)
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, "new msg 100", err.Error())

	errTr := ErrorAsTranslatable(err)
	assert.Equal(t, "EOF", errTr.Error())
	wrapped, ok = errTr.(*errorTranslatableWrapper)
	require.True(t, ok)
	assert.Equal(t, "key", wrapped.trKey)
}
