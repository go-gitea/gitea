// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

type trieEdge struct {
	b    byte
	node *TrieNode
}

// TrieNode represents a node in a slice-based Trie for fast byte-sequence prefix matching.
type TrieNode struct {
	children []trieEdge
	isEnd    bool
}

func (t *TrieNode) child(b byte) *TrieNode {
	for _, edge := range t.children {
		if edge.b == b {
			return edge.node
		}
	}
	return nil
}

// Insert adds a string (byte sequence) to the Trie.
func (t *TrieNode) Insert(val string) {
	curr := t
	for i := 0; i < len(val); i++ {
		next := curr.child(val[i])
		if next == nil {
			next = &TrieNode{}
			curr.children = append(curr.children, trieEdge{b: val[i], node: next})
		}
		curr = next
	}
	curr.isEnd = true
}

// Match returns the length of the longest matching prefix starting at index `start` in string `s`.
// It returns -1 if no prefix is matched.
func (t *TrieNode) Match(s string, start int) int {
	curr := t
	matchLen := -1
	for j := start; j < len(s); j++ {
		next := curr.child(s[j])
		if next == nil {
			break
		}
		curr = next
		if curr.isEnd {
			matchLen = j - start + 1
		}
	}
	return matchLen
}
