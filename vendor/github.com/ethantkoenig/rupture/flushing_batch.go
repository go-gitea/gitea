package rupture

import (
	"github.com/blevesearch/bleve"
)

// FlushingBatch is a batch of operations that automatically flushes to the
// underlying index once it reaches a certain size.
type FlushingBatch interface {
	// Index adds the specified index operation batch, possibly triggering a
	// flush.
	Index(id string, data interface{}) error
	// Remove adds the specified delete operation to the batch, possibly
	// triggering a flush.
	Delete(id string) error
	// Flush flushes the batch's contents.
	Flush() error
}

type singleIndexFlushingBatch struct {
	maxBatchSize int
	batch        *bleve.Batch
	index        bleve.Index
}

func newFlushingBatch(index bleve.Index, maxBatchSize int) *singleIndexFlushingBatch {
	return &singleIndexFlushingBatch{
		maxBatchSize: maxBatchSize,
		batch:        index.NewBatch(),
		index:        index,
	}
}

// NewFlushingBatch creates a new flushing batch for the specified index. Once
// the number of operations in the batch reaches the specified limit, the batch
// automatically flushes its operations to the index.
func NewFlushingBatch(index bleve.Index, maxBatchSize int) FlushingBatch {
	return newFlushingBatch(index, maxBatchSize)
}

func (b *singleIndexFlushingBatch) Index(id string, data interface{}) error {
	if err := b.batch.Index(id, data); err != nil {
		return err
	}
	return b.flushIfFull()
}

func (b *singleIndexFlushingBatch) Delete(id string) error {
	b.batch.Delete(id)
	return b.flushIfFull()
}

func (b *singleIndexFlushingBatch) flushIfFull() error {
	if b.batch.Size() < b.maxBatchSize {
		return nil
	}
	return b.Flush()
}

func (b *singleIndexFlushingBatch) Flush() error {
	err := b.index.Batch(b.batch)
	if err != nil {
		return err
	}
	b.batch.Reset()
	return nil
}
