// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

// ByteFIFO defines a FIFO that takes a byte array
type ByteFIFO interface {
	// Len returns the length of the fifo
	Len() int64
	// PushFunc pushes data to the end of the fifo and calls the callback if it is added
	PushFunc(data []byte, fn func() error) error
	// Pop pops data from the start of the fifo
	Pop() ([]byte, error)
	// Close this fifo
	Close() error
}

// UniqueByteFIFO defines a FIFO that Uniques its contents
type UniqueByteFIFO interface {
	ByteFIFO
	// Has returns whether the fifo contains this data
	Has(data []byte) (bool, error)
}

var _ ByteFIFO = &DummyByteFIFO{}

// DummyByteFIFO represents a dummy fifo
type DummyByteFIFO struct{}

// PushFunc returns nil
func (*DummyByteFIFO) PushFunc(data []byte, fn func() error) error {
	return nil
}

// Pop returns nil
func (*DummyByteFIFO) Pop() ([]byte, error) {
	return []byte{}, nil
}

// Close returns nil
func (*DummyByteFIFO) Close() error {
	return nil
}

// Len is always 0
func (*DummyByteFIFO) Len() int64 {
	return 0
}

var _ UniqueByteFIFO = &DummyUniqueByteFIFO{}

// DummyUniqueByteFIFO represents a dummy unique fifo
type DummyUniqueByteFIFO struct {
	DummyByteFIFO
}

// Has always returns false
func (*DummyUniqueByteFIFO) Has([]byte) (bool, error) {
	return false, nil
}
