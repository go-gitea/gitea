//  Copyright (c) 2017 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zap

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/Smerity/govarint"
)

type chunkedIntCoder struct {
	final     []byte
	maxDocNum uint64
	chunkSize uint64
	chunkBuf  bytes.Buffer
	encoder   *govarint.Base128Encoder
	chunkLens []uint64
	currChunk uint64

	buf []byte
}

// newChunkedIntCoder returns a new chunk int coder which packs data into
// chunks based on the provided chunkSize and supports up to the specified
// maxDocNum
func newChunkedIntCoder(chunkSize uint64, maxDocNum uint64) *chunkedIntCoder {
	total := maxDocNum/chunkSize + 1
	rv := &chunkedIntCoder{
		chunkSize: chunkSize,
		maxDocNum: maxDocNum,
		chunkLens: make([]uint64, total),
		final:     make([]byte, 0, 64),
	}
	rv.encoder = govarint.NewU64Base128Encoder(&rv.chunkBuf)

	return rv
}

// Reset lets you reuse this chunked int coder.  buffers are reset and reused
// from previous use.  you cannot change the chunk size or max doc num.
func (c *chunkedIntCoder) Reset() {
	c.final = c.final[:0]
	c.chunkBuf.Reset()
	c.currChunk = 0
	for i := range c.chunkLens {
		c.chunkLens[i] = 0
	}
}

// Add encodes the provided integers into the correct chunk for the provided
// doc num.  You MUST call Add() with increasing docNums.
func (c *chunkedIntCoder) Add(docNum uint64, vals ...uint64) error {
	chunk := docNum / c.chunkSize
	if chunk != c.currChunk {
		// starting a new chunk
		if c.encoder != nil {
			// close out last
			c.Close()
			c.chunkBuf.Reset()
		}
		c.currChunk = chunk
	}

	for _, val := range vals {
		_, err := c.encoder.PutU64(val)
		if err != nil {
			return err
		}
	}

	return nil
}

// Close indicates you are done calling Add() this allows the final chunk
// to be encoded.
func (c *chunkedIntCoder) Close() {
	c.encoder.Close()
	encodingBytes := c.chunkBuf.Bytes()
	c.chunkLens[c.currChunk] = uint64(len(encodingBytes))
	c.final = append(c.final, encodingBytes...)
}

// Write commits all the encoded chunked integers to the provided writer.
func (c *chunkedIntCoder) Write(w io.Writer) (int, error) {
	bufNeeded := binary.MaxVarintLen64 * (1 + len(c.chunkLens))
	if len(c.buf) < bufNeeded {
		c.buf = make([]byte, bufNeeded)
	}
	buf := c.buf

	// write out the number of chunks & each chunkLen
	n := binary.PutUvarint(buf, uint64(len(c.chunkLens)))
	for _, chunkLen := range c.chunkLens {
		n += binary.PutUvarint(buf[n:], uint64(chunkLen))
	}

	tw, err := w.Write(buf[:n])
	if err != nil {
		return tw, err
	}

	// write out the data
	nw, err := w.Write(c.final)
	tw += nw
	if err != nil {
		return tw, err
	}
	return tw, nil
}
