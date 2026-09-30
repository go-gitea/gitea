// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
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

func TestErrorUnwrapForUser(t *testing.T) {
	err := NewNotExistErrorf("test msg")
	msg, code := ErrorUnwrapForUser(err)
	assert.Equal(t, "test msg", msg)
	assert.Equal(t, 404, code)

	err = fmt.Errorf("other wrapper: %w", err)
	msg, code = ErrorUnwrapForUser(err)
	assert.Equal(t, "other wrapper: test msg", msg)
	assert.Equal(t, 404, code)

	msg, code = ErrorUnwrapForUser(io.EOF)
	assert.Equal(t, "", msg)
	assert.Equal(t, 0, code)
}
