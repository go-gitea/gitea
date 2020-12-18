package trie

type Node struct {
	Char     rune
	Children []*Node
	End      bool
}

func NewNode(char rune) *Node {
	return &Node{Char: char, End: false}
}

func (node *Node) get(char rune) *Node {
	for _, child := range node.Children {
		if char == child.Char {
			return child
		}
	}

	return nil
}

func (node *Node) put(char rune) *Node {
	child := node.get(char)

	if child == nil {
		child = NewNode(char)
		node.Children = append(node.Children, child)
	}

	return child
}