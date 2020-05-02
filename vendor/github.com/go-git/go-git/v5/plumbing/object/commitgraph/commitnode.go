package commitgraph

import (
	"io"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// CommitNode is generic interface encapsulating a lightweight commit object retrieved
// from CommitNodeIndex
type CommitNode interface {
	// ID returns the Commit object id referenced by the commit graph node.
	ID() plumbing.Hash
	// Tree returns the Tree referenced by the commit graph node.
	Tree() (*object.Tree, error)
	// CommitTime returns the Commiter.When time of the Commit referenced by the commit graph node.
	CommitTime() time.Time
	// NumParents returns the number of parents in a commit.
	NumParents() int
	// ParentNodes return a CommitNodeIter for parents of specified node.
	ParentNodes() CommitNodeIter
	// ParentNode returns the ith parent of a commit.
	ParentNode(i int) (CommitNode, error)
	// ParentHashes returns hashes of the parent commits for a specified node
	ParentHashes() []plumbing.Hash
	// Generation returns the generation of the commit for reachability analysis.
	// Objects with newer generation are not reachable from objects of older generation.
	Generation() uint64
	// Commit returns the full commit object from the node
	Commit() (*object.Commit, error)
}

// CommitNodeIndex is generic interface encapsulating an index of CommitNode objects
type CommitNodeIndex interface {
	// Get returns a commit node from a commit hash
	Get(hash plumbing.Hash) (CommitNode, error)
}

// CommitNodeIter is a generic closable interface for iterating over commit nodes.
type CommitNodeIter interface {
	Next() (CommitNode, error)
	ForEach(func(CommitNode) error) error
	Close()
}

// parentCommitNodeIter provides an iterator for parent commits from associated CommitNodeIndex.
type parentCommitNodeIter struct {
	node CommitNode
	i    int
}

func newParentgraphCommitNodeIter(node CommitNode) CommitNodeIter {
	return &parentCommitNodeIter{node, 0}
}

// Next moves the iterator to the next commit and returns a pointer to it. If
// there are no more commits, it returns io.EOF.
func (iter *parentCommitNodeIter) Next() (CommitNode, error) {
	obj, err := iter.node.ParentNode(iter.i)
	if err == object.ErrParentNotFound {
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
