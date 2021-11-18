// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewImapClient(t *testing.T) {
	_, err := NewImapClient(ClientInitOpt{
		Addr:     "127.0.0.1:1179",
		IsTLS:    false,
		UserName: "receive@gitea.io",
		Passwd:   "123456",
	})

	assert.NoError(t, err)
}

func TestGetUnreadMailIDs(t *testing.T) {
	c, err := NewImapClient(ClientInitOpt{
		Addr:     "127.0.0.1:1179",
		IsTLS:    false,
		UserName: "receive@gitea.io",
		Passwd:   "123456",
	})
	assert.NoError(t, err)

	ms, err := c.GetUnreadMailIDs("INBOX")
	assert.NoError(t, err)
	assert.EqualValues(t, ms, []uint32{1})
}

