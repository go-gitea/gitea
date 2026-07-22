// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

// TrieEdge represents a byte transition edge in the Trie.
type TrieEdge struct {
	B    byte
	Node *TrieNode
}

// TrieNode represents a node in a slice-based Trie for fast byte-sequence prefix matching.
type TrieNode struct {
	Children []TrieEdge
	IsEnd    bool
}

// Insert adds a string (byte sequence) to the Trie.
func (t *TrieNode) Insert(val string) {
	curr := t
	for i := 0; i < len(val); i++ {
		b := val[i]
		var next *TrieNode
		for _, edge := range curr.Children {
			if edge.B == b {
				next = edge.Node
				break
			}
		}
		if next == nil {
			next = &TrieNode{}
			curr.Children = append(curr.Children, TrieEdge{B: b, Node: next})
		}
		curr = next
	}
	curr.IsEnd = true
}

// Match returns the length of the longest matching prefix starting at index `start` in string `s`.
// It returns -1 if no prefix is matched.
func (t *TrieNode) Match(s string, start int) int {
	curr := t
	matchLen := -1
	for j := start; j < len(s); j++ {
		b := s[j]
		var next *TrieNode
		for _, edge := range curr.Children {
			if edge.B == b {
				next = edge.Node
				break
			}
		}
		if next == nil {
			break
		}
		curr = next
		if curr.IsEnd {
			matchLen = j - start + 1
		}
	}
	return matchLen
}
