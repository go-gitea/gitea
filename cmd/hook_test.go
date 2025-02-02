// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPktLine(t *testing.T) {
	// test read
	ctx := context.Background()
	s := strings.NewReader("0000")
	r := bufio.NewReader(s)
	result, err := readPktLine(ctx, r, pktLineTypeFlush)
	assert.NoError(t, err)
	assert.Equal(t, pktLineTypeFlush, result.Type)

	s = strings.NewReader("0006a\n")
	r = bufio.NewReader(s)
	result, err = readPktLine(ctx, r, pktLineTypeData)
	assert.NoError(t, err)
	assert.Equal(t, pktLineTypeData, result.Type)
	assert.Equal(t, []byte("a\n"), result.Data)

	// test write
	w := bytes.NewBuffer([]byte{})
	err = writeFlushPktLine(ctx, w)
	assert.NoError(t, err)
	assert.Equal(t, []byte("0000"), w.Bytes())

	w.Reset()
	err = writeDataPktLine(ctx, w, []byte("a\nb"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("0007a\nb"), w.Bytes())
}
