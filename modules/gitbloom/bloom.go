package gitbloom

import (
	"github.com/spaolacci/murmur3"
)

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
	hash0 := murmur3.Sum32WithSeed([]byte(path), 0x293ae76f)
	hash1 := murmur3.Sum32WithSeed([]byte(path), 0x7e646e2c)
	for i := uint32(0); i < 7; i++ {
		bit := (hash0 + hash1*i) % uint32(len(f.b)*8)
		if f.b[bit>>3]&(1<<(bit&7)) == 0 {
			return false
		}
	}
	return true
}

// Add path data to the filter.
func (f *BloomPathFilter) Add(path string) {
	hash0 := murmur3.Sum32WithSeed([]byte(path), 0x293ae76f)
	hash1 := murmur3.Sum32WithSeed([]byte(path), 0x7e646e2c)
	for i := uint32(0); i < 7; i++ {
		bit := (hash0 + hash1*i) % uint32(len(f.b)*8)
		f.b[bit>>3] |= 1 << (bit & 7)
	}
}

// Data returns data bytes
func (f *BloomPathFilter) Data() []byte {
	return f.b
}

// NewBloomPathFilter creates a new empty bloom filter for n changed paths
func NewBloomPathFilter(n int) *BloomPathFilter {
	f := &BloomPathFilter{make([]byte, ((n+63)/64)*8)}
	return f
}

// LoadBloomPathFilter creates a bloom filter from a byte array previously
// returned by Data
func LoadBloomPathFilter(data []byte) *BloomPathFilter {
	f := &BloomPathFilter{data}
	return f
}
