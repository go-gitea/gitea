package transfer

import (
	"encoding/hex"
	"hash"
	"io"
)

var _ io.Reader = (*HashingReader)(nil)

// HashingReader is a reader that hashes the data it reads.
type HashingReader struct {
	r    io.Reader
	hash hash.Hash
	size int64
}

// NewHashingReader creates a new hashing reader.
func NewHashingReader(r io.Reader, hash hash.Hash) *HashingReader {
	return &HashingReader{
		r:    r,
		hash: hash,
	}
}

// Size returns the number of bytes read.
func (h *HashingReader) Size() int64 {
	return h.size
}

// Oid returns the hash of the data read.
func (h *HashingReader) Oid() string {
	return hex.EncodeToString(h.hash.Sum(nil))
}

// Read reads data from the underlying reader and hashes it.
func (h *HashingReader) Read(p []byte) (int, error) {
	n, err := h.r.Read(p)
	h.size += int64(n)
	h.hash.Write(p[:n])
	return n, err
}
