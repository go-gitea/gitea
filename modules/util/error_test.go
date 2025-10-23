// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorTranslatable(t *testing.T) {
	var err error

	err = ErrorWrapTranslatable(io.EOF, "key", 1)
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, "EOF", err.Error())
	assert.Equal(t, "key", err.(*errorTranslatableWrapper).trKey)
	assert.Equal(t, []any{1}, err.(*errorTranslatableWrapper).trArgs)

	err = ErrorWrap(err, "new msg %d", 100)
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, "new msg 100", err.Error())

	errTr := ErrorAsTranslatable(err)
	assert.Equal(t, "EOF", errTr.Error())
	assert.Equal(t, "key", errTr.(*errorTranslatableWrapper).trKey)
}
