package diff

// DifferenceLocation indicates where the difference occurs
type DifferenceLocation struct {
	URL      string `json:"url"`
	Method   string `json:"method,omitempty"`
	Response int    `json:"response,omitempty"`
	Node     *Node  `json:"node,omitempty"`
}

// AddNode returns a copy of this location with the leaf node added
func (dl DifferenceLocation) AddNode(node *Node) DifferenceLocation {
	newLoc := dl

	if newLoc.Node != nil {
		newLoc.Node = newLoc.Node.Copy()
		newLoc.Node.AddLeafNode(node)
	} else {
		newLoc.Node = node
	}
	return newLoc
}
