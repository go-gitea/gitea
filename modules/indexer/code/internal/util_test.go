// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseKeywordAsPhrase(t *testing.T) {
	phrase, isPhrase := ParseKeywordAsPhrase(`a`)
	assert.Empty(t, phrase)
	assert.False(t, isPhrase)

	phrase, isPhrase = ParseKeywordAsPhrase(`"a"`)
	assert.Equal(t, "a", phrase)
	assert.True(t, isPhrase)

	phrase, isPhrase = ParseKeywordAsPhrase(`""\"""`)
	assert.Equal(t, `"\""`, phrase)
	assert.True(t, isPhrase)
}
