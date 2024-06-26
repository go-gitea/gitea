package transfer

import (
	"encoding/hex"
	"fmt"
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

var _ io.Reader = (*VerifyingReader)(nil)

// VerifyingReader is a reader that hashes the data it reads.
// At EOF, it compares the OID and size with expected values and replaces EOF
// with an error in the case of mismatch.
type VerifyingReader struct {
	r            *HashingReader
	expectedOid  string
	expectedSize int64
}

// NewVerifyingReader creates a new VerifyingReader.
func NewVerifyingReader(r *HashingReader, oid string, size int64) *VerifyingReader {
	return &VerifyingReader{
		r:            r,
		expectedOid:  oid,
		expectedSize: size,
	}
}

// Read reads data from the underlying HashingReader.
// At EOF, it compares results and returns error if OID or size mismatch
func (v *VerifyingReader) Read(p []byte) (int, error) {
	n, err := v.r.Read(p)
	if err == io.EOF {
		// stream consumed, now check for mismatch
		if v.r.Size() != v.expectedSize {
			err = fmt.Errorf("%w: invalid object size, expected %v, got %v", ErrCorruptData, v.expectedSize, v.r.Size())
		}
		if v.r.Oid() != v.expectedOid {
			err = fmt.Errorf("%w: invalid object ID, expected %v, got %v", ErrCorruptData, v.expectedOid, v.r.Oid())
		}
	}
	return n, err
}
