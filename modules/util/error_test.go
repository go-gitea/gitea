// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"html/template"
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
	wrapped, ok := err.(*errorTranslatable)
	require.True(t, ok)
	assert.Equal(t, "key", wrapped.trKey)
	assert.Equal(t, []any{1}, wrapped.trArgs)

	err = ErrorWrap(err, "new msg %d", 100)
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, "new msg 100", err.Error())
}

type testErrorLocale struct{}

func (l testErrorLocale) Tr(key string, args ...any) template.HTML {
	return template.HTML(l.TrString(key, args...))
}

func (testErrorLocale) TrString(key string, args ...any) string {
	return fmt.Sprintf(key, args...)
}

func TestErrorUnwrapForHTML(t *testing.T) {
	locale := testErrorLocale{}
	err := NewNotExistErrorf("missing <file>")
	msg, code := ErrorUnwrapForHTML(err, locale)
	assert.Equal(t, template.HTML("missing &lt;file&gt;"), msg)
	assert.Equal(t, 404, code)

	msg, code = ErrorUnwrapForHTML(ErrorWrapTranslatable(err, "missing <b>%s</b>", "file"), locale)
	assert.Equal(t, template.HTML("missing <b>file</b>"), msg)
	assert.Equal(t, 404, code)

	msg, code = ErrorUnwrapForHTML(ErrorWrapTranslatable(io.EOF, "unexpected end of input"), locale)
	assert.Equal(t, template.HTML("unexpected end of input"), msg)
	assert.Equal(t, 400, code)

	msg, code = ErrorUnwrapForHTML(io.EOF, locale)
	assert.Empty(t, msg)
	assert.Zero(t, code)
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
