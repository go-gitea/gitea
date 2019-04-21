package commitgraph

import (
	"bytes"
	"crypto/sha1"
	"hash"
	"io"
	"math"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/utils/binary"
)

// Encoder writes MemoryIndex structs to an output stream.
type Encoder struct {
	io.Writer
	hash hash.Hash
}

// NewEncoder returns a new stream encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	h := sha1.New()
	mw := io.MultiWriter(w, h)
	return &Encoder{mw, h}
}

func (e *Encoder) Encode(idx Index) error {
	// Get all the hashes in the memory index
	hashes := idx.Hashes()

	// Sort the hashes and build our index
	plumbing.HashesSort(hashes)
	hashToIndex := make(map[plumbing.Hash]uint32)
	hashFirstToCount := make(map[byte]uint32)
	for i, hash := range hashes {
		hashToIndex[hash] = uint32(i)
		hashFirstToCount[hash[0]]++
	}

	// Find out if we will need large edge table
	chunkCount := 3
	hasLargeEdges := false
	for i := 0; i < len(hashes); i++ {
		v, _ := idx.GetNodeByIndex(i)
		if len(v.ParentHashes) > 2 {
			hasLargeEdges = true
			chunkCount++
			break
		}
	}

	// Find out if the bloom filters are present
	hasBloomFilters := false
	sparseBloomFilters := false
	bloomFiltersCount := 0
	for i := 0; i < len(hashes); i++ {
		_, err := idx.GetBloomFilterByIndex(i)
		if err == nil {
			bloomFiltersCount++
		}
	}
	if bloomFiltersCount > 0 {
		hasBloomFilters = true
		chunkCount++
		if bloomFiltersCount < (len(hashes) * 4 / 3) {
			sparseBloomFilters = true
			chunkCount++
		}
	}

	var fanoutOffset = uint64(20 + (chunkCount * 12))
	var oidLookupOffset = fanoutOffset + 4*256
	var commitDataOffset = oidLookupOffset + uint64(len(hashes))*20
	var bloomOffset = commitDataOffset + uint64(len(hashes))*36
	var sparseBloomOffset = bloomOffset + uint64(bloomFiltersCount)*640
	var largeEdgeListOffset = bloomOffset
	var largeEdges []uint32

	// Write header
	// TODO: Error handling
	e.Write(commitFileSignature)
	e.Write([]byte{1, 1, byte(chunkCount), 0})

	// Write chunk headers
	e.Write(oidFanoutSignature)
	binary.WriteUint64(e, fanoutOffset)
	e.Write(oidLookupSignature)
	binary.WriteUint64(e, oidLookupOffset)
	e.Write(commitDataSignature)
	binary.WriteUint64(e, commitDataOffset)
	if hasBloomFilters {
		e.Write(experimentalBloomSignature)
		binary.WriteUint64(e, bloomOffset)
		if sparseBloomFilters {
			e.Write(experimentalSparseBloomSignature)
			binary.WriteUint64(e, sparseBloomOffset)
			largeEdgeListOffset = sparseBloomOffset + uint64(len(hashes)+7)/8
		} else {
			largeEdgeListOffset = bloomOffset + 640*uint64(len(hashes))
		}
	}
	if hasLargeEdges {
		e.Write(largeEdgeListSignature)
		binary.WriteUint64(e, largeEdgeListOffset)
	}
	e.Write([]byte{0, 0, 0, 0})
	binary.WriteUint64(e, uint64(0))

	// Write fanout
	var cumulative uint32
	for i := 0; i <= 0xff; i++ {
		if err := binary.WriteUint32(e, hashFirstToCount[byte(i)]+cumulative); err != nil {
			return err
		}
		cumulative += hashFirstToCount[byte(i)]
	}

	// Write OID lookup
	for _, hash := range hashes {
		if _, err := e.Write(hash[:]); err != nil {
			return err
		}
	}

	// Write commit data
	for _, hash := range hashes {
		origIndex, _ := idx.GetIndexByHash(hash)
		commitData, _ := idx.GetNodeByIndex(origIndex)
		if _, err := e.Write(commitData.TreeHash[:]); err != nil {
			return err
		}

		if len(commitData.ParentHashes) == 0 {
			binary.WriteUint32(e, parentNone)
			binary.WriteUint32(e, parentNone)
		} else if len(commitData.ParentHashes) == 1 {
			binary.WriteUint32(e, hashToIndex[commitData.ParentHashes[0]])
			binary.WriteUint32(e, parentNone)
		} else if len(commitData.ParentHashes) == 2 {
			binary.WriteUint32(e, hashToIndex[commitData.ParentHashes[0]])
			binary.WriteUint32(e, hashToIndex[commitData.ParentHashes[1]])
		} else if len(commitData.ParentHashes) > 2 {
			binary.WriteUint32(e, hashToIndex[commitData.ParentHashes[0]])
			binary.WriteUint32(e, uint32(len(largeEdges))|parentOctopusMask)
			for _, parentHash := range commitData.ParentHashes[1:] {
				largeEdges = append(largeEdges, hashToIndex[parentHash])
			}
			largeEdges[len(largeEdges)-1] |= parentLast
		}

		unixTime := uint64(commitData.When.Unix())
		unixTime |= uint64(commitData.Generation) << 34
		binary.WriteUint64(e, unixTime)
	}

	// Write bloom filters (experimental)
	if hasBloomFilters {
		var sparseBloomBitset []byte

		if sparseBloomFilters {
			sparseBloomBitset = bytes.Repeat([]byte{0xff}, (len(hashes)+7)/8)
		}

		for i, hash := range hashes {
			origIndex, _ := idx.GetIndexByHash(hash)
			if bloomFilter, err := idx.GetBloomFilterByIndex(origIndex); err != nil {
				if !sparseBloomFilters {
					for i := 0; i < 80; i++ {
						binary.WriteUint64(e, math.MaxUint64)
					}
				} else {
					sparseBloomBitset[i/8] &= ^(1 << uint(i%8))
				}
			} else {
				e.Write(bloomFilter.Data())
			}
		}

		if sparseBloomFilters {
			e.Write(sparseBloomBitset)
		}
	}

	// Write large edges if necessary
	if hasLargeEdges {
		for _, parent := range largeEdges {
			binary.WriteUint32(e, parent)
		}
	}

	// Write checksum
	if _, err := e.Write(e.hash.Sum(nil)[:20]); err != nil {
		return err
	}

	return nil
}
