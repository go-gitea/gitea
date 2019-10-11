package diff

// Node is the position od a diff in a spec
type Node struct {
	Field     string `json:"name,omitempty"`
	TypeName  string `json:"type,omitempty"`
	IsArray   bool   `json:"is_array,omitempty"`
	ChildNode *Node  `json:"child,omitempty"`
}

// String std string render
func (n *Node) String() string {
	name := n.Field
	if n.IsArray {
		name = "array[" + n.TypeName + "]"
	}

	if n.ChildNode != nil {
		return name + "." + n.ChildNode.String()
	}
	if len(n.TypeName) > 0 {
		return name + " : " + n.TypeName
	}
	return name
}

// AddLeafNode Adds (recursive) a Child to the first non-nil child found
func (n *Node) AddLeafNode(toAdd *Node) *Node {

	if n.ChildNode == nil {
		n.ChildNode = toAdd
	} else {
		n.ChildNode.AddLeafNode(toAdd)
	}

	return n
}

//Copy deep copy of this node and children
func (n Node) Copy() *Node {
	newNode := n

	if newNode.ChildNode != nil {
		n.ChildNode = newNode.ChildNode.Copy()
	}
	return &newNode
}
