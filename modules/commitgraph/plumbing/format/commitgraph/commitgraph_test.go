package commitgraph_test

import (
	"testing"

	"code.gitea.io/gitea/modules/commitgraph/plumbing/format/commitgraph"
	"golang.org/x/exp/mmap"

	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git-fixtures.v3"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func Test(t *testing.T) { TestingT(t) }

type CommitgraphSuite struct {
	fixtures.Suite
}

var _ = Suite(&CommitgraphSuite{})

func (s *CommitgraphSuite) TestDecode(c *C) {
	reader, err := mmap.Open("..\\..\\tests\\testgit\\objects\\info\\commit-graph")
	c.Assert(err, IsNil)
	index, err := commitgraph.OpenFileIndex(reader)
	c.Assert(err, IsNil)

	nodeIndex, err := index.GetIndexByHash(plumbing.NewHash("5aa811d3c2f6d5d6e928a4acacd15248928c26d0"))
	c.Assert(err, IsNil)
	node, err := index.GetNodeByIndex(nodeIndex)
	c.Assert(err, IsNil)
	c.Assert(len(node.ParentIndexes), Equals, 0)

	reader.Close()
}
