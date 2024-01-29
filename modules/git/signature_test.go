// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewNewSignatureFromCommitline(t *testing.T) {
	tz := time.FixedZone("", 2*60*60)

	kases := map[string]Signature{
		"": {},
		"author gitea test <test@gitea.com>": {
			Name:  "author gitea test",
			Email: "test@gitea.com",
		},
		"author gitea test <test@gitea.com> 1705912028 +0200": {
			Name:  "author gitea test",
			Email: "test@gitea.com",
			When:  time.Unix(1705912028, 0).In(tz),
		},
		"author gitea test <test@gitea.com> Mon Jan 22 10:27:08 2024 +0200": {
			Name:  "author gitea test",
			Email: "test@gitea.com",
			When:  time.Unix(1705912028, 0).In(tz),
		},
	}

	for text, sign := range kases {
		newSign, err := newSignatureFromCommitline([]byte(text))
		assert.NoError(t, err)
		assert.Equal(t, sign.Name, newSign.Name)
		assert.Equal(t, sign.Email, newSign.Email)
		assert.Equal(t, sign.When, newSign.When)
	}
}
