package commitgraph

import (
	"fmt"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/commitgraph"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// graphCommitNode is a reduced representation of Commit as presented in the commit
// graph file (commitgraph.Node). It is merely useful as an optimization for walking
// the commit graphs.
//
// graphCommitNode implements the CommitNode interface.
type graphCommitNode struct {
	// Hash for the Commit object
	hash plumbing.Hash
	// Index of the node in the commit graph file
	index int

	commitData *commitgraph.CommitData
	gci        *graphCommitNodeIndex
}

// graphCommitNodeIndex is an index that can load CommitNode objects from both the commit
// graph files and the object store.
//
// graphCommitNodeIndex implements the CommitNodeIndex interface
type graphCommitNodeIndex struct {
	commitGraph commitgraph.Index
	s           storer.EncodedObjectStorer
}

// NewGraphCommitNodeIndex returns CommitNodeIndex implementation that uses commit-graph
// files as backing storage and falls back to object storage when necessary
func NewGraphCommitNodeIndex(commitGraph commitgraph.Index, s storer.EncodedObjectStorer) CommitNodeIndex {
	return &graphCommitNodeIndex{commitGraph, s}
}

func (gci *graphCommitNodeIndex) Get(hash plumbing.Hash) (CommitNode, error) {
	// Check the commit graph first
	parentIndex, err := gci.commitGraph.GetIndexByHash(hash)
	if err == nil {
		parent, err := gci.commitGraph.GetCommitDataByIndex(parentIndex)
		if err != nil {
			return nil, err
		}

		return &graphCommitNode{
			hash:       hash,
			index:      parentIndex,
			commitData: parent,
			gci:        gci,
		}, nil
	}

	// Fallback to loading full commit object
	commit, err := object.GetCommit(gci.s, hash)
	if err != nil {
		return nil, err
	}

	return &objectCommitNode{
		nodeIndex: gci,
		commit:    commit,
	}, nil
}

func (c *graphCommitNode) ID() plumbing.Hash {
	return c.hash
}

func (c *graphCommitNode) Tree() (*object.Tree, error) {
	return object.GetTree(c.gci.s, c.commitData.TreeHash)
}

func (c *graphCommitNode) CommitTime() time.Time {
	return c.commitData.When
}

func (c *graphCommitNode) NumParents() int {
	return len(c.commitData.ParentIndexes)
}

func (c *graphCommitNode) ParentNodes() CommitNodeIter {
	return newParentgraphCommitNodeIter(c)
}

func (c *graphCommitNode) ParentNode(i int) (CommitNode, error) {
	if i < 0 || i >= len(c.commitData.ParentIndexes) {
		return nil, object.ErrParentNotFound
	}

	parent, err := c.gci.commitGraph.GetCommitDataByIndex(c.commitData.ParentIndexes[i])
	if err != nil {
		return nil, err
	}

	return &graphCommitNode{
		hash:       c.commitData.ParentHashes[i],
		index:      c.commitData.ParentIndexes[i],
		commitData: parent,
		gci:        c.gci,
	}, nil
}

func (c *graphCommitNode) ParentHashes() []plumbing.Hash {
	return c.commitData.ParentHashes
}

func (c *graphCommitNode) Generation() uint64 {
	// If the commit-graph file was generated with older Git version that
	// set the generation to zero for every commit the generation assumption
	// is still valid. It is just less useful.
	return uint64(c.commitData.Generation)
}

func (c *graphCommitNode) Commit() (*object.Commit, error) {
	return object.GetCommit(c.gci.s, c.hash)
}

func (c *graphCommitNode) String() string {
	return fmt.Sprintf(
		"%s %s\nDate:   %s",
		plumbing.CommitObject, c.ID(),
		c.CommitTime().Format(object.DateFormat),
	)
}
