// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseKeywordAsPhrase(t *testing.T) {
	cases := []struct {
		keyword  string
		phrase   string
		isPhrase bool
	}{
		{``, "", false},
		{`a`, "", false},
		{`"`, "", false},
		{`"a`, "", false},
		{`"a"`, "a", true},
		{`""\"""`, `"\""`, true},
	}
	for _, c := range cases {
		phrase, isPhrase := ParseKeywordAsPhrase(c.keyword)
		assert.Equal(t, c.phrase, phrase, "keyword=%q", c.keyword)
		assert.Equal(t, c.isPhrase, isPhrase, "keyword=%q", c.keyword)
	}
}
