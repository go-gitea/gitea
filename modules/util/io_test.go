// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type readerWithError struct {
	buf *bytes.Buffer
}

func (r *readerWithError) Read(p []byte) (n int, err error) {
	if r.buf.Len() < 2 {
		return 0, errors.New("test error")
	}
	return r.buf.Read(p)
}

func TestReadWithLimit(t *testing.T) {
	bs := []byte("0123456789abcdef")

	// normal test
	buf, err := readWithLimit(bytes.NewBuffer(bs), 5, 2)
	assert.NoError(t, err)
	assert.Equal(t, []byte("01"), buf)

	buf, err = readWithLimit(bytes.NewBuffer(bs), 5, 5)
	assert.NoError(t, err)
	assert.Equal(t, []byte("01234"), buf)

	buf, err = readWithLimit(bytes.NewBuffer(bs), 5, 6)
	assert.NoError(t, err)
	assert.Equal(t, []byte("012345"), buf)

	buf, err = readWithLimit(bytes.NewBuffer(bs), 5, len(bs))
	assert.NoError(t, err)
	assert.Equal(t, []byte("0123456789abcdef"), buf)

	buf, err = readWithLimit(bytes.NewBuffer(bs), 5, 100)
	assert.NoError(t, err)
	assert.Equal(t, []byte("0123456789abcdef"), buf)

	// test with error
	buf, err = readWithLimit(&readerWithError{bytes.NewBuffer(bs)}, 5, 10)
	assert.NoError(t, err)
	assert.Equal(t, []byte("0123456789"), buf)

	buf, err = readWithLimit(&readerWithError{bytes.NewBuffer(bs)}, 5, 100)
	assert.ErrorContains(t, err, "test error")
	assert.Empty(t, buf)

	// test public function
	buf, err = ReadWithLimit(bytes.NewBuffer(bs), 2)
	assert.NoError(t, err)
	assert.Equal(t, []byte("01"), buf)

	buf, err = ReadWithLimit(bytes.NewBuffer(bs), 9999999)
	assert.NoError(t, err)
	assert.Equal(t, []byte("0123456789abcdef"), buf)
}
