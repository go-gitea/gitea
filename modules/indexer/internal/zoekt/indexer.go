package zoekt

import (
	"context"
	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/shards"
)

type Indexer struct {
	indexDir string
	Searcher zoekt.Streamer
}

func NewIndexer(indexDir string) *Indexer {
	return &Indexer{
		indexDir: indexDir,
	}
}

func (i *Indexer) Init(ctx context.Context) (bool, error) {
	searcher, err := shards.NewDirectorySearcher(i.indexDir)
	if err != nil {
		return false, err
	}
	i.Searcher = searcher

	return true, nil
}

func (i *Indexer) Ping(ctx context.Context) error {
	// NOTHING TO DO
	return nil
}

func (i *Indexer) Close() {
	// NOTHING TO DO
}
