package git

import (
	"os"
	"path"

	"gopkg.in/src-d/go-git.v4/plumbing/format/commitgraph"
	cgobject "gopkg.in/src-d/go-git.v4/plumbing/object/commitgraph"
)

// CommitNodeIndex returns the index for walking commit graph
func (r *Repository) CommitNodeIndex() (cgobject.CommitNodeIndex, *os.File) {
	indexPath := path.Join(r.Path, "objects", "info", "commit-graph")

	file, err := os.Open(indexPath)
	if err == nil {
		index, err := commitgraph.OpenFileIndex(file)
		if err == nil {
			return cgobject.NewGraphCommitNodeIndex(index, r.gogitRepo.Storer), file
		}
	}

	return cgobject.NewObjectCommitNodeIndex(r.gogitRepo.Storer), nil
}
