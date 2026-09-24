// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrie(t *testing.T) {
	trie := &TrieNode{}
	trie.Insert("apple")
	trie.Insert("apricot")
	trie.Insert("banana")
	trie.Insert("app")

	// Test exact matches
	assert.Equal(t, 5, trie.Match("apple", 0, nil))
	assert.Equal(t, 7, trie.Match("apricot", 0, nil))
	assert.Equal(t, 6, trie.Match("banana", 0, nil))
	assert.Equal(t, 3, trie.Match("app", 0, nil))

	// Test partial match (longest match priority)
	assert.Equal(t, 5, trie.Match("apple-pie", 0, nil))

	// Test suffix/nested position match
	assert.Equal(t, 5, trie.Match("sweet apple", 6, nil))

	// Test no match
	assert.Equal(t, -1, trie.Match("orange", 0, nil))
	assert.Equal(t, -1, trie.Match("ap", 0, nil)) // prefix exists but is not an end node

	skipDash := func(s string, pos int) int { return Iif(s[pos] == '-', 1, 0) }
	assert.Equal(t, 9, trie.Match("a-p-p-l-e", 0, skipDash))
	assert.Equal(t, 5, trie.Match("app--x", 0, skipDash))
	assert.Equal(t, -1, trie.Match("-apple", 0, skipDash))
	assert.Equal(t, -1, trie.Match("ap-", 0, skipDash))
}
