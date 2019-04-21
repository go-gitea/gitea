package object

import (
	"fmt"
	"io"
	"time"

	"code.gitea.io/gitea/modules/commitgraph/plumbing/format/commitgraph"

	"gopkg.in/src-d/go-git.v4/plumbing"
	ggobject "gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

// CommitNode is generic interface encapsulating either Commit object or
// graphCommitNode object
type CommitNode interface {
	ID() plumbing.Hash
	Tree() (*ggobject.Tree, error)
	CommitTime() time.Time
}

// CommitNodeIndex is generic interface encapsulating an index of CommitNode objects
// and accessor methods for walking it as a directed graph
type CommitNodeIndex interface {
	NumParents(node CommitNode) int
	ParentNodes(node CommitNode) CommitNodeIter
	ParentNode(node CommitNode, i int) (CommitNode, error)
	ParentHashes(node CommitNode) []plumbing.Hash

	NodeFromHash(hash plumbing.Hash) (CommitNode, error)

	// Commit returns the full commit object from the node
	Commit(node CommitNode) (*ggobject.Commit, error)

	// BloomFilter returns optional bloom filter for changed file paths
	BloomFilter(node CommitNode) (*commitgraph.BloomPathFilter, error)
}

// CommitNodeIter is a generic closable interface for iterating over commit nodes.
type CommitNodeIter interface {
	Next() (CommitNode, error)
	ForEach(func(CommitNode) error) error
	Close()
}

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

	node *commitgraph.Node
	gci  *graphCommitNodeIndex
}

// graphCommitNodeIndex is an index that can load CommitNode objects from both the commit
// graph files and the object store.
//
// graphCommitNodeIndex implements the CommitNodeIndex interface
type graphCommitNodeIndex struct {
	commitGraph commitgraph.Index
	s           storer.EncodedObjectStorer
}

// objectCommitNode is a representation of Commit as presented in the GIT object format.
//
// objectCommitNode implements the CommitNode interface.
type objectCommitNode struct {
	commit *ggobject.Commit
}

// objectCommitNodeIndex is an index that can load CommitNode objects only from the
// object store.
//
// objectCommitNodeIndex implements the CommitNodeIndex interface
type objectCommitNodeIndex struct {
	s storer.EncodedObjectStorer
}

// ID returns the Commit object id referenced by the commit graph node.
func (c *graphCommitNode) ID() plumbing.Hash {
	return c.hash
}

// Tree returns the Tree referenced by the commit graph node.
func (c *graphCommitNode) Tree() (*ggobject.Tree, error) {
	return ggobject.GetTree(c.gci.s, c.node.TreeHash)
}

// CommitTime returns the Commiter.When time of the Commit referenced by the commit graph node.
func (c *graphCommitNode) CommitTime() time.Time {
	return c.node.When
}

func (c *graphCommitNode) String() string {
	return fmt.Sprintf(
		"%s %s\nDate:   %s",
		plumbing.CommitObject, c.ID(),
		c.CommitTime().Format(ggobject.DateFormat),
	)
}

func NewGraphCommitNodeIndex(commitGraph commitgraph.Index, s storer.EncodedObjectStorer) CommitNodeIndex {
	return &graphCommitNodeIndex{commitGraph, s}
}

// NumParents returns the number of parents in a commit.
func (gci *graphCommitNodeIndex) NumParents(node CommitNode) int {
	if cgn, ok := node.(*graphCommitNode); ok {
		return len(cgn.node.ParentIndexes)
	}
	co := node.(*objectCommitNode)
	return co.commit.NumParents()
}

// ParentNodes return a CommitNodeIter for parents of specified node.
func (gci *graphCommitNodeIndex) ParentNodes(node CommitNode) CommitNodeIter {
	return newParentgraphCommitNodeIter(gci, node)
}

// ParentNode returns the ith parent of a commit.
func (gci *graphCommitNodeIndex) ParentNode(node CommitNode, i int) (CommitNode, error) {
	if cgn, ok := node.(*graphCommitNode); ok {
		if len(cgn.node.ParentIndexes) == 0 || i >= len(cgn.node.ParentIndexes) {
			return nil, ggobject.ErrParentNotFound
		}

		parent, err := gci.commitGraph.GetNodeByIndex(cgn.node.ParentIndexes[i])
		if err != nil {
			return nil, err
		}

		return &graphCommitNode{
			hash:  cgn.node.ParentHashes[i],
			index: cgn.node.ParentIndexes[i],
			node:  parent,
			gci:   gci,
		}, nil
	}

	co := node.(*objectCommitNode)
	if len(co.commit.ParentHashes) == 0 || i >= len(co.commit.ParentHashes) {
		return nil, ggobject.ErrParentNotFound
	}

	parentHash := co.commit.ParentHashes[i]
	return gci.NodeFromHash(parentHash)
}

// ParentHashes returns hashes of the parent commits for a specified node
func (gci *graphCommitNodeIndex) ParentHashes(node CommitNode) []plumbing.Hash {
	if cgn, ok := node.(*graphCommitNode); ok {
		return cgn.node.ParentHashes
	}
	co := node.(*objectCommitNode)
	return co.commit.ParentHashes
}

// NodeFromHash looks up a commit node by it's object hash
func (gci *graphCommitNodeIndex) NodeFromHash(hash plumbing.Hash) (CommitNode, error) {
	// Check the commit graph first
	parentIndex, err := gci.commitGraph.GetIndexByHash(hash)
	if err == nil {
		parent, err := gci.commitGraph.GetNodeByIndex(parentIndex)
		if err != nil {
			return nil, err
		}

		return &graphCommitNode{
			hash:  hash,
			index: parentIndex,
			node:  parent,
			gci:   gci,
		}, nil
	}

	// Fallback to loading full commit object
	commit, err := ggobject.GetCommit(gci.s, hash)
	if err != nil {
		return nil, err
	}

	return &objectCommitNode{commit: commit}, nil
}

// Commit returns the full Commit object representing the commit graph node.
func (gci *graphCommitNodeIndex) Commit(node CommitNode) (*ggobject.Commit, error) {
	if cgn, ok := node.(*graphCommitNode); ok {
		return ggobject.GetCommit(gci.s, cgn.ID())
	}
	co := node.(*objectCommitNode)
	return co.commit, nil
}

// BloomFilter returns optional bloom filter for changed file paths
func (gci *graphCommitNodeIndex) BloomFilter(node CommitNode) (*commitgraph.BloomPathFilter, error) {
	if cgn, ok := node.(*graphCommitNode); ok {
		return gci.commitGraph.GetBloomFilterByIndex(cgn.index)
	}
	return nil, plumbing.ErrObjectNotFound
}

// CommitTime returns the time when the commit was performed.
//
// CommitTime is present to fulfill the CommitNode interface.
func (c *objectCommitNode) CommitTime() time.Time {
	return c.commit.Committer.When
}

// ID returns the Commit object id referenced by the node.
func (c *objectCommitNode) ID() plumbing.Hash {
	return c.commit.ID()
}

// Tree returns the Tree referenced by the node.
func (c *objectCommitNode) Tree() (*ggobject.Tree, error) {
	return c.commit.Tree()
}

func NewObjectCommitNodeIndex(s storer.EncodedObjectStorer) CommitNodeIndex {
	return &objectCommitNodeIndex{s}
}

// NumParents returns the number of parents in a commit.
func (oci *objectCommitNodeIndex) NumParents(node CommitNode) int {
	co := node.(*objectCommitNode)
	return co.commit.NumParents()
}

// ParentNodes return a CommitNodeIter for parents of specified node.
func (oci *objectCommitNodeIndex) ParentNodes(node CommitNode) CommitNodeIter {
	return newParentgraphCommitNodeIter(oci, node)
}

// ParentNode returns the ith parent of a commit.
func (oci *objectCommitNodeIndex) ParentNode(node CommitNode, i int) (CommitNode, error) {
	co := node.(*objectCommitNode)
	parent, err := co.commit.Parent(i)
	if err != nil {
		return nil, err
	}
	return &objectCommitNode{commit: parent}, nil
}

// ParentHashes returns hashes of the parent commits for a specified node
func (oci *objectCommitNodeIndex) ParentHashes(node CommitNode) []plumbing.Hash {
	co := node.(*objectCommitNode)
	return co.commit.ParentHashes
}

// NodeFromHash looks up a commit node by it's object hash
func (oci *objectCommitNodeIndex) NodeFromHash(hash plumbing.Hash) (CommitNode, error) {
	commit, err := ggobject.GetCommit(oci.s, hash)
	if err != nil {
		return nil, err
	}

	return &objectCommitNode{commit: commit}, nil
}

// Commit returns the full Commit object representing the commit graph node.
func (oci *objectCommitNodeIndex) Commit(node CommitNode) (*ggobject.Commit, error) {
	co := node.(*objectCommitNode)
	return co.commit, nil
}

// BloomFilter returns optional bloom filter for changed file paths
func (oci *objectCommitNodeIndex) BloomFilter(node CommitNode) (*commitgraph.BloomPathFilter, error) {
	return nil, plumbing.ErrObjectNotFound
}

// parentCommitNodeIter provides an iterator for parent commits from associated CommitNodeIndex.
type parentCommitNodeIter struct {
	gci  CommitNodeIndex
	node CommitNode
	i    int
}

func newParentgraphCommitNodeIter(gci CommitNodeIndex, node CommitNode) CommitNodeIter {
	return &parentCommitNodeIter{gci, node, 0}
}

// Next moves the iterator to the next commit and returns a pointer to it. If
// there are no more commits, it returns io.EOF.
func (iter *parentCommitNodeIter) Next() (CommitNode, error) {
	obj, err := iter.gci.ParentNode(iter.node, iter.i)
	if err == ggobject.ErrParentNotFound {
		return nil, io.EOF
	}
	if err == nil {
		iter.i++
	}

	return obj, err
}

// ForEach call the cb function for each commit contained on this iter until
// an error appends or the end of the iter is reached. If ErrStop is sent
// the iteration is stopped but no error is returned. The iterator is closed.
func (iter *parentCommitNodeIter) ForEach(cb func(CommitNode) error) error {
	for {
		obj, err := iter.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		if err := cb(obj); err != nil {
			if err == storer.ErrStop {
				return nil
			}

			return err
		}
	}
}

func (iter *parentCommitNodeIter) Close() {
}
