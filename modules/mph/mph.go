// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package mph

import (
	"sort"

	"github.com/spaolacci/murmur3"
)

// A ConstructedHashFunction holds information about the built perfect hash function.
type ConstructedHashFunction struct {
	levelZeroBuckets []uint32
	levelZeroMask    int
	levelOneEntries  []uint32
	levelOneMask     int
}

// Build builds a  perfect hash function from keys using the "Hash, displace, and compress".
// Given the use-case of this
// Ref: http://cmph.sourceforge.net/papers/esa09.pdf.
func Build(keys []string) *ConstructedHashFunction {
	// These values are not described in the paper as the paper allows any universal hash
	// function(with certain bounds) in which case we use murmur3 and mask to avoid "overflow",
	// this could be replaced by module but this is a more performant easy "hack" to do this.

	// Construct values for the first level of hash function.
	// This is used to map the strings to `nextPow2(len(keys)/3)` amount of buckets.
	// This seems to be across benchmarks one of faster values for the use-case.
	levelZeroBuckets := make([]uint32, nextPow2(len(keys)/3))
	levelZeroMask := len(levelZeroBuckets) - 1
	// Construct values for the second level of hash functions.
	// This is used for the hash function to find the index within a specific bucket.
	levelOneEntries := make([]uint32, nextPow2(len(keys)))
	levelOneMask := len(levelOneEntries) - 1

	// Create temporary buckets.
	tempBuckets := make([][]int, len(levelZeroBuckets))

	// Construct a simple perfect hash function. Not every bucket would be used,
	// which is fine as we will fix that when we transform it into a compressed form
	// and thereby making it a minimal perfect hash function.
	for i, s := range keys {
		n := int(murmur3.Sum32([]byte(s))) & levelZeroMask
		tempBuckets[n] = append(tempBuckets[n], i)
	}

	// Covert it to indexBuckets which can be sorted on the bucket's amount of values.
	// We also hereby filter out the empty bucket.
	var buckets indexBuckets
	for n, vals := range tempBuckets {
		if len(vals) > 0 {
			buckets = append(buckets, indexBucket{n, vals})
		}
	}

	// Sort the buckets by their size in descending order.
	sort.Sort(buckets)

	// Now go trough each bucket and kind of brute force your way into making a perfect hash again.
	// We want to find the displace value here which is a seed value which makes the hash function within
	// the bucker a perfect hash function(no collision between the keys).

	// Store for each entry within a bucket if a key has hashed to that entry.
	tempEvaluatedIdx := make([]bool, len(levelOneEntries))

	// Store all hashed indexes, so it's easier to clean the variable up.
	var evaluatedIndexs []int
	for _, bucket := range buckets {
		// Always start from zero and work your way up their.
		seed := uint32(0)
	trySeed:
		// Reset temporary hashed indexes.
		evaluatedIndexs = evaluatedIndexs[:0]
		for _, i := range bucket.values {
			// Create the hash for this value.
			n := int(murmur3.Sum32WithSeed([]byte(keys[i]), seed)) & levelOneMask
			// Check if this hash is a collision.
			if tempEvaluatedIdx[n] {
				// A collision, reset everything and try a new seed.
				for _, n := range evaluatedIndexs {
					tempEvaluatedIdx[n] = false
				}
				seed++
				goto trySeed
			}
			// Mark this index has being used.
			tempEvaluatedIdx[n] = true

			// Add the index to the evaluated indexes.
			evaluatedIndexs = append(evaluatedIndexs, n)

			// This somehow doesn't cause conflicts. So we just leave it here.
			levelOneEntries[n] = uint32(i)
		}

		// No collisions detected, save this seed for this bucket.
		levelZeroBuckets[bucket.originialIdx] = seed
	}

	// Return the table.
	return &ConstructedHashFunction{
		levelZeroBuckets: levelZeroBuckets,
		levelZeroMask:    levelZeroMask,
		levelOneEntries:  levelOneEntries,
		levelOneMask:     levelOneMask,
	}
}

// General purpose fast method of finding the next power of two.
// Unless Go decides to expose more specialized bit methods to find the
// first one in a number, this is as best we get without hacking around.
func nextPow2(n int) int {
	for i := 1; ; i <<= 1 {
		if i >= n {
			return i
		}
	}
}

// Get searches for s in t and returns its index.
func (chf *ConstructedHashFunction) Get(s string) uint32 {
	// Find the bucket the key is stored in.
	bucketIdx := int(murmur3.Sum32([]byte(s))) & chf.levelZeroMask
	seed := chf.levelZeroBuckets[bucketIdx]
	// Get the index within the bucket.
	idx := int(murmur3.Sum32WithSeed([]byte(s), seed)) & chf.levelOneMask
	return chf.levelOneEntries[idx]
}

type indexBucket struct {
	originialIdx int
	values       []int
}

type indexBuckets []indexBucket

func (s indexBuckets) Len() int           { return len(s) }
func (s indexBuckets) Less(a, z int) bool { return len(s[a].values) > len(s[z].values) }
func (s indexBuckets) Swap(a, z int)      { s[a], s[z] = s[z], s[a] }
