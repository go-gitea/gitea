package commitgraph

import (
	"bytes"
	"errors"
	"io"
	"math"
	"time"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/utils/binary"
)

var (
	// ErrUnsupportedVersion is returned by OpenFileIndex when the commit graph
	// file version is not supported.
	ErrUnsupportedVersion = errors.New("Unsuported version")
	// ErrUnsupportedHash is returned by OpenFileIndex when the commit graph
	// hash function is not supported. Currently only SHA-1 is defined and
	// supported
	ErrUnsupportedHash = errors.New("Unsuported hash algorithm")
	// ErrMalformedCommitGraphFile is returned by OpenFileIndex when the commit
	// graph file is corrupted.
	ErrMalformedCommitGraphFile = errors.New("Malformed commit graph file")

	commitFileSignature              = []byte{'C', 'G', 'P', 'H'}
	oidFanoutSignature               = []byte{'O', 'I', 'D', 'F'}
	oidLookupSignature               = []byte{'O', 'I', 'D', 'L'}
	commitDataSignature              = []byte{'C', 'D', 'A', 'T'}
	largeEdgeListSignature           = []byte{'E', 'D', 'G', 'E'}
	experimentalBloomSignature       = []byte{'X', 'G', 'G', 'B'}
	experimentalSparseBloomSignature = []byte{'X', 'G', 'S', 'B'}

	parentNone        = uint32(0x70000000)
	parentOctopusUsed = uint32(0x80000000)
	parentOctopusMask = uint32(0x7fffffff)
	parentLast        = uint32(0x80000000)
)

type fileIndex struct {
	reader              io.ReaderAt
	fanout              [256]int
	oidLookupOffset     int64
	commitDataOffset    int64
	largeEdgeListOffset int64
	bloomOffset         int64
	sparseBloomOffset   int64
	sparseBloomMap      map[int]int
}

// OpenFileIndex opens a serialized commit graph file in the format described at
// https://github.com/git/git/blob/master/Documentation/technical/commit-graph-format.txt
func OpenFileIndex(reader io.ReaderAt) (Index, error) {
	// Verify file signature
	var signature = make([]byte, 4)
	if _, err := reader.ReadAt(signature, 0); err != nil {
		return nil, err
	}
	if !bytes.Equal(signature, commitFileSignature) {
		return nil, ErrMalformedCommitGraphFile
	}

	// Read and verify the file header
	var header = make([]byte, 4)
	if _, err := reader.ReadAt(header, 4); err != nil {
		return nil, err
	}
	if header[0] != 1 {
		return nil, ErrUnsupportedVersion
	}
	if header[1] != 1 {
		return nil, ErrUnsupportedHash
	}

	// Read chunk headers
	var chunkID = make([]byte, 4)
	var oidFanoutOffset int64
	var oidLookupOffset int64
	var commitDataOffset int64
	var largeEdgeListOffset int64
	var bloomOffset int64
	var sparseBloomOffset int64
	chunkCount := int(header[2])
	for i := 0; i < chunkCount; i++ {
		chunkHeader := io.NewSectionReader(reader, 8+(int64(i)*12), 12)
		if _, err := io.ReadAtLeast(chunkHeader, chunkID, 4); err != nil {
			return nil, err
		}
		chunkOffset, err := binary.ReadUint64(chunkHeader)
		if err != nil {
			return nil, err
		}

		if bytes.Equal(chunkID, oidFanoutSignature) {
			oidFanoutOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, oidLookupSignature) {
			oidLookupOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, commitDataSignature) {
			commitDataOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, largeEdgeListSignature) {
			largeEdgeListOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, experimentalBloomSignature) {
			bloomOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, experimentalSparseBloomSignature) {
			sparseBloomOffset = int64(chunkOffset)
		}
	}

	if oidFanoutOffset <= 0 || oidLookupOffset <= 0 || commitDataOffset <= 0 {
		return nil, ErrMalformedCommitGraphFile
	}

	// Read fanout table and calculate the file offsets into the lookup table
	fanoutReader := io.NewSectionReader(reader, oidFanoutOffset, 256*4)
	var fanout [256]int
	for i := 0; i < 256; i++ {
		fanoutValue, err := binary.ReadUint32(fanoutReader)
		if err != nil {
			return nil, err
		}
		if fanoutValue > 0x7fffffff {
			return nil, ErrMalformedCommitGraphFile
		}
		fanout[i] = int(fanoutValue)
	}

	return &fileIndex{reader, fanout, oidLookupOffset, commitDataOffset, largeEdgeListOffset, bloomOffset, sparseBloomOffset, nil}, nil
}

func (fi *fileIndex) GetIndexByHash(h plumbing.Hash) (int, error) {
	var oid plumbing.Hash

	// Find the hash in the oid lookup table
	var low int
	if h[0] == 0 {
		low = 0
	} else {
		low = fi.fanout[h[0]-1]
	}
	high := fi.fanout[h[0]]
	for low < high {
		mid := (low + high) >> 1
		offset := fi.oidLookupOffset + int64(mid)*20
		if _, err := fi.reader.ReadAt(oid[:], offset); err != nil {
			return 0, err
		}
		cmp := bytes.Compare(h[:], oid[:])
		if cmp < 0 {
			high = mid
		} else if cmp == 0 {
			return mid, nil
		} else {
			low = mid + 1
		}
	}

	return 0, plumbing.ErrObjectNotFound
}

func (fi *fileIndex) GetNodeByIndex(idx int) (*Node, error) {
	if idx >= fi.fanout[0xff] {
		return nil, plumbing.ErrObjectNotFound
	}

	offset := fi.commitDataOffset + int64(idx)*36
	commitDataReader := io.NewSectionReader(fi.reader, offset, 36)

	treeHash, err := binary.ReadHash(commitDataReader)
	if err != nil {
		return nil, err
	}
	parent1, err := binary.ReadUint32(commitDataReader)
	if err != nil {
		return nil, err
	}
	parent2, err := binary.ReadUint32(commitDataReader)
	if err != nil {
		return nil, err
	}
	genAndTime, err := binary.ReadUint64(commitDataReader)
	if err != nil {
		return nil, err
	}

	var parentIndexes []int
	if parent2&parentOctopusUsed == parentOctopusUsed {
		// Octopus merge
		parentIndexes = []int{int(parent1 & parentOctopusMask)}
		offset := fi.largeEdgeListOffset + 4*int64(parent2&parentOctopusMask)
		parentReader := io.NewSectionReader(fi.reader, offset, math.MaxInt64)
		for {
			parent, err := binary.ReadUint32(parentReader)
			if err != nil {
				return nil, err
			}
			parentIndexes = append(parentIndexes, int(parent&parentOctopusMask))
			if parent&parentLast == parentLast {
				break
			}
		}
	} else if parent2 != parentNone {
		parentIndexes = []int{int(parent1 & parentOctopusMask), int(parent2 & parentOctopusMask)}
	} else if parent1 != parentNone {
		parentIndexes = []int{int(parent1 & parentOctopusMask)}
	}

	parentHashes, err := fi.getHashesFromIndexes(parentIndexes)
	if err != nil {
		return nil, err
	}

	return &Node{
		TreeHash:      treeHash,
		ParentIndexes: parentIndexes,
		ParentHashes:  parentHashes,
		Generation:    int(genAndTime >> 34),
		When:          time.Unix(int64(genAndTime&0x3FFFFFFFF), 0),
	}, nil
}

func (fi *fileIndex) getHashesFromIndexes(indexes []int) ([]plumbing.Hash, error) {
	hashes := make([]plumbing.Hash, len(indexes))

	for i, idx := range indexes {
		if idx > fi.fanout[0xff] {
			return nil, ErrMalformedCommitGraphFile
		}

		offset := fi.oidLookupOffset + int64(idx)*20
		if _, err := fi.reader.ReadAt(hashes[i][:], offset); err != nil {
			return nil, err
		}
	}

	return hashes, nil
}

// Hashes returns all the hashes that are available in the index
func (fi *fileIndex) Hashes() []plumbing.Hash {
	hashes := make([]plumbing.Hash, 0, fi.fanout[0xff])
	for i := 0; i < int(fi.fanout[0xff]); i++ {
		offset := fi.oidLookupOffset + int64(i)*20
		if _, err := fi.reader.ReadAt(hashes[i][:], offset); err != nil {
			return nil
		}
	}
	return hashes
}

// GetBloomFilterByIndex gets the bloom filter for files changed in the commit, if available
func (fi *fileIndex) GetBloomFilterByIndex(i int) (*BloomPathFilter, error) {
	if fi.bloomOffset == 0 || i >= fi.fanout[0xff] {
		return nil, plumbing.ErrObjectNotFound
	}

	if fi.sparseBloomOffset != 0 {
		if fi.sparseBloomMap != nil {
			// Build the map
			fi.sparseBloomMap = make(map[int]int)
			sparseBloomSize := (fi.fanout[0xff] + 7) / 8
			sparseBloomReader := io.NewSectionReader(fi.reader, fi.sparseBloomOffset, int64(sparseBloomSize))
			sparseBloomBits := make([]byte, sparseBloomSize)
			if _, err := io.ReadAtLeast(sparseBloomReader, sparseBloomBits, sparseBloomSize); err != nil {
				return nil, err
			}

			for idx, sidx := 0, 0; idx < sparseBloomSize; idx++ {
				if sparseBloomBits[idx]&1 > 0 {
					fi.sparseBloomMap[idx*8] = sidx
					sidx++
				}
				if sparseBloomBits[idx]&2 > 0 {
					fi.sparseBloomMap[idx*8+1] = sidx
					sidx++
				}
				if sparseBloomBits[idx]&4 > 0 {
					fi.sparseBloomMap[idx*8+2] = sidx
					sidx++
				}
				if sparseBloomBits[idx]&8 > 0 {
					fi.sparseBloomMap[idx*8+3] = sidx
					sidx++
				}
				if sparseBloomBits[idx]&16 > 0 {
					fi.sparseBloomMap[idx*8+4] = sidx
					sidx++
				}
				if sparseBloomBits[idx]&32 > 0 {
					fi.sparseBloomMap[idx*8+5] = sidx
					sidx++
				}
				if sparseBloomBits[idx]&64 > 0 {
					fi.sparseBloomMap[idx*8+6] = sidx
					sidx++
				}
				if sparseBloomBits[idx]&128 > 0 {
					fi.sparseBloomMap[idx*8+7] = sidx
					sidx++
				}
			}
		}

		// Map the original index into the sparse one
		var ok bool
		if i, ok = fi.sparseBloomMap[i]; !ok {
			return nil, plumbing.ErrObjectNotFound
		}
	}

	offset := fi.bloomOffset + int64(i)*640
	bloomReader := io.NewSectionReader(fi.reader, offset, 640)
	bloomBits := make([]byte, 640)
	if _, err := io.ReadAtLeast(bloomReader, bloomBits, 640); err != nil {
		return nil, err
	}
	return LoadBloomPathFilter(bloomBits), nil
}
