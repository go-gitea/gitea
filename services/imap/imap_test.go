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

func TestMail_LoadHeader(t *testing.T) {
	c, err := NewImapClient(ClientInitOpt{
		Addr:     "127.0.0.1:1179",
		IsTLS:    false,
		UserName: "receive@gitea.io",
		Passwd:   "123456",
	})
	assert.NoError(t, err)

	ms, err := c.GetUnreadMails("INBOX", 5)
	assert.NoError(t, err)
	if !assert.Equal(t, len(ms), 1) {
		return
	}

	assert.NoError(t, ms[0].LoadHeader([]string{"Message-ID"}))
	assert.Equal(t, ms[0].Heads["Message-ID"][0].Address, "0000000@localhost")
}

func TestMail_LoadBody(t *testing.T) {
	c, err := NewImapClient(ClientInitOpt{
		Addr:     "127.0.0.1:1179",
		IsTLS:    false,
		UserName: "receive@gitea.io",
		Passwd:   "123456",
	})
	assert.NoError(t, err)

	ms, err := c.GetUnreadMails("INBOX", 5)
	assert.NoError(t, err)
	if !assert.Equal(t, len(ms), 1) {
		return
	}

	assert.NoError(t, ms[0].LoadBody())
	assert.Equal(t, ms[0].ContentText, "Hi there :)")
}
