// Package trie is an implementation of a trie (prefix tree) data structure over byte slices. It provides a
// small and simple API for usage as a set as well as a 'Node' API for walking the trie.
package trie

// A Trie is a a prefix tree.
type Trie struct {
	root *Node
}

// New construct a new, empty Trie ready for use.
func New() *Trie {
	return &Trie{
		root: &Node{},
	}
}

// Insert puts b into the Trie. It returns true if the element was not previously in t.
func (t *Trie) Insert(b []byte) bool {
	n := t.root
	for _, c := range b {
		next, ok := n.Walk(c)
		if !ok {
			next = &Node{}
			n.branches[c] = next
			n.hasChildren = true
		}
		n = next
	}
	if n.terminal {
		return false
	}
	n.terminal = true
	return true
}

// Contains checks t for membership of b.
func (t *Trie) Contains(b []byte) bool {
	n := t.root
	for _, c := range b {
		next, ok := n.Walk(c)
		if !ok {
			return false
		}
		n = next
	}
	return n.terminal
}

// PrefixIndex walks through `b` until a prefix is found (terminal node) or it is exhausted.
func (t *Trie) PrefixIndex(b []byte) int {
	var idx int
	n := t.root
	for _, c := range b {
		next, ok := n.Walk(c)
		if !ok {
			return -1
		}
		if next.terminal {
			return idx
		}
		n = next
		idx++
	}
	if !n.terminal {
		idx = -1
	}
	return idx
}

// Root returns the root node of a Trie. A valid Trie (i.e., constructed with New), always has a non-nil root
// node.
func (t *Trie) Root() *Node {
	return t.root
}

// A Node represents a logical vertex in the trie structure.
type Node struct {
	branches    [256]*Node
	terminal    bool
	hasChildren bool
}

// Walk returns the node reached along edge c, if one exists. The ok value indicates whether such a node
// exist.
func (n *Node) Walk(c byte) (next *Node, ok bool) {
	next = n.branches[int(c)]
	return next, (next != nil)
}

// Terminal indicates whether n is terminal in the trie (that is, whether the path from the root to n
// represents an element in the set). For instance, if the root node is terminal, then []byte{} is in the
// trie.
func (n *Node) Terminal() bool {
	return n.terminal
}

// Leaf indicates whether n is a leaf node in the trie (that is, whether it has children). A leaf node must be
// terminal (else it would not exist). Logically, if n is a leaf node then the []byte represented by the path
// from the root to n is not a proper prefix of any element of the trie.
func (n *Node) Leaf() bool {
	return !n.hasChildren
}
