package diff

import (
	"fmt"

	"github.com/go-openapi/spec"
)

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
		name = fmt.Sprintf("%s<array[%s]>", name, n.TypeName)
	} else if len(n.TypeName) > 0 {
		name = fmt.Sprintf("%s<%s>", name, n.TypeName)
	}
	if n.ChildNode != nil {
		return fmt.Sprintf("%s.%s", name, n.ChildNode.String())
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

// Copy deep copy of this node and children
func (n Node) Copy() *Node {
	newChild := n.ChildNode
	if newChild != nil {
		newChild = newChild.Copy()
	}
	newNode := Node{
		Field:     n.Field,
		TypeName:  n.TypeName,
		IsArray:   n.IsArray,
		ChildNode: newChild,
	}

	return &newNode
}

func getSchemaDiffNode(name string, schema interface{}) *Node {
	node := Node{
		Field: name,
	}
	if schema != nil {
		switch s := schema.(type) {
		case spec.Refable:
			node.TypeName, node.IsArray = getSchemaType(s)
		case *spec.Schema:
			node.TypeName, node.IsArray = getSchemaType(s.SchemaProps)
		case spec.SimpleSchema:
			node.TypeName, node.IsArray = getSchemaType(s)
		case *spec.SimpleSchema:
			node.TypeName, node.IsArray = getSchemaType(s)
		case *spec.SchemaProps:
			node.TypeName, node.IsArray = getSchemaType(s)
		case spec.SchemaProps:
			node.TypeName, node.IsArray = getSchemaType(&s)
		default:
			node.TypeName = fmt.Sprintf("Unknown type %v", schema)
		}
	}
	return &node
}
