package commitgraph

import (
	"encoding/binary"
	"hash"
	"hash/fnv"

	"github.com/dchest/siphash"
)

type filter struct {
	m uint32
	k uint32
	h hash.Hash64
}

func (f *filter) bits(data []byte) []uint32 {
	f.h.Reset()
	f.h.Write(data)
	d := f.h.Sum(nil)
	a := binary.BigEndian.Uint32(d[4:8])
	b := binary.BigEndian.Uint32(d[0:4])
	is := make([]uint32, f.k)
	for i := uint32(0); i < f.k; i++ {
		is[i] = (a + b*i) % f.m
	}
	return is
}

func newFilter(m, k uint32) *filter {
	return &filter{
		m: m,
		k: k,
		h: fnv.New64(),
	}
}

// BloomPathFilter is a probabilistic data structure that helps determining
// whether a path was was changed.
//
// The implementation uses a standard bloom filter with n=512, m=10, k=7
// parameters using the 64-bit SipHash hash function with zero key.
type BloomPathFilter struct {
	b []byte
}

// Test checks whether a path was previously added to the filter. Returns
// false if the path is not present in the filter. Returns true if the path
// could be present in the filter.
func (f *BloomPathFilter) Test(path string) bool {
	d := siphash.Hash(0, 0, []byte(path))
	a := uint32(d)
	b := uint32(d >> 32)
	var i uint32
	for i = 0; i < 7; i++ {
		bit := (a + b*i) % 5120
		if f.b[bit>>3]&(1<<(bit&7)) == 0 {
			return false
		}
	}
	return true
}

// Add path data to the filter.
func (f *BloomPathFilter) Add(path string) {
	d := siphash.Hash(0, 0, []byte(path))
	a := uint32(d)
	b := uint32(d >> 32)
	var i uint32
	for i = 0; i < 7; i++ {
		bit := (a + b*i) % 5120
		f.b[bit>>3] |= 1 << (bit & 7)
	}
}

// Data returns data bytes
func (f *BloomPathFilter) Data() []byte {
	return f.b
}

// NewBloomPathFilter creates a new empty bloom filter
func NewBloomPathFilter() *BloomPathFilter {
	f := &BloomPathFilter{make([]byte, 640)}
	return f
}

// LoadBloomPathFilter creates a bloom filter from a byte array previously
// returned by Data
func LoadBloomPathFilter(data []byte) *BloomPathFilter {
	f := &BloomPathFilter{data}
	return f
}
