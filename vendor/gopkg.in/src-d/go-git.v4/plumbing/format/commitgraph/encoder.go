package commitgraph

import (
	"crypto/sha1"
	"hash"
	"io"

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

// Encode writes an index into the commit-graph file
func (e *Encoder) Encode(idx Index) error {
	// Get all the hashes in the input index
	hashes := idx.Hashes()

	// Sort the inout and prepare helper structures we'll need for encoding
	hashToIndex, fanout, extraEdgesCount := e.prepare(idx, hashes)

	chunkSignatures := [][]byte{oidFanoutSignature, oidLookupSignature, commitDataSignature}
	chunkSizes := []uint64{4 * 256, uint64(len(hashes)) * 20, uint64(len(hashes)) * 36}
	if extraEdgesCount > 0 {
		chunkSignatures = append(chunkSignatures, extraEdgeListSignature)
		chunkSizes = append(chunkSizes, uint64(extraEdgesCount)*4)
	}

	if err := e.encodeFileHeader(len(chunkSignatures)); err != nil {
		return err
	}
	if err := e.encodeChunkHeaders(chunkSignatures, chunkSizes); err != nil {
		return err
	}
	if err := e.encodeFanout(fanout); err != nil {
		return err
	}
	if err := e.encodeOidLookup(hashes); err != nil {
		return err
	}
	if extraEdges, err := e.encodeCommitData(hashes, hashToIndex, idx); err == nil {
		if err = e.encodeExtraEdges(extraEdges); err != nil {
			return err
		}
	} else {
		return err
	}

	return e.encodeChecksum()
}

func (e *Encoder) prepare(idx Index, hashes []plumbing.Hash) (hashToIndex map[plumbing.Hash]uint32, fanout []uint32, extraEdgesCount uint32) {
	// Sort the hashes and build our index
	plumbing.HashesSort(hashes)
	hashToIndex = make(map[plumbing.Hash]uint32)
	fanout = make([]uint32, 256)
	for i, hash := range hashes {
		hashToIndex[hash] = uint32(i)
		fanout[hash[0]]++
	}

	// Convert the fanout to cumulative values
	for i := 1; i <= 0xff; i++ {
		fanout[i] += fanout[i-1]
	}

	// Find out if we will need extra edge table
	for i := 0; i < len(hashes); i++ {
		v, _ := idx.GetCommitDataByIndex(i)
		if len(v.ParentHashes) > 2 {
			extraEdgesCount += uint32(len(v.ParentHashes) - 1)
			break
		}
	}

	return
}

func (e *Encoder) encodeFileHeader(chunkCount int) (err error) {
	if _, err = e.Write(commitFileSignature); err == nil {
		_, err = e.Write([]byte{1, 1, byte(chunkCount), 0})
	}
	return
}

func (e *Encoder) encodeChunkHeaders(chunkSignatures [][]byte, chunkSizes []uint64) (err error) {
	// 8 bytes of file header, 12 bytes for each chunk header and 12 byte for terminator
	offset := uint64(8 + len(chunkSignatures)*12 + 12)
	for i, signature := range chunkSignatures {
		if _, err = e.Write(signature); err == nil {
			err = binary.WriteUint64(e, offset)
		}
		if err != nil {
			return
		}
		offset += chunkSizes[i]
	}
	if _, err = e.Write(lastSignature); err == nil {
		err = binary.WriteUint64(e, offset)
	}
	return
}

func (e *Encoder) encodeFanout(fanout []uint32) (err error) {
	for i := 0; i <= 0xff; i++ {
		if err = binary.WriteUint32(e, fanout[i]); err != nil {
			return
		}
	}
	return
}

func (e *Encoder) encodeOidLookup(hashes []plumbing.Hash) (err error) {
	for _, hash := range hashes {
		if _, err = e.Write(hash[:]); err != nil {
			return err
		}
	}
	return
}

func (e *Encoder) encodeCommitData(hashes []plumbing.Hash, hashToIndex map[plumbing.Hash]uint32, idx Index) (extraEdges []uint32, err error) {
	for _, hash := range hashes {
		origIndex, _ := idx.GetIndexByHash(hash)
		commitData, _ := idx.GetCommitDataByIndex(origIndex)
		if _, err = e.Write(commitData.TreeHash[:]); err != nil {
			return
		}

		var parent1, parent2 uint32
		if len(commitData.ParentHashes) == 0 {
			parent1 = parentNone
			parent2 = parentNone
		} else if len(commitData.ParentHashes) == 1 {
			parent1 = hashToIndex[commitData.ParentHashes[0]]
			parent2 = parentNone
		} else if len(commitData.ParentHashes) == 2 {
			parent1 = hashToIndex[commitData.ParentHashes[0]]
			parent2 = hashToIndex[commitData.ParentHashes[1]]
		} else if len(commitData.ParentHashes) > 2 {
			parent1 = hashToIndex[commitData.ParentHashes[0]]
			parent2 = uint32(len(extraEdges)) | parentOctopusUsed
			for _, parentHash := range commitData.ParentHashes[1:] {
				extraEdges = append(extraEdges, hashToIndex[parentHash])
			}
			extraEdges[len(extraEdges)-1] |= parentLast
		}

		if err = binary.WriteUint32(e, parent1); err == nil {
			err = binary.WriteUint32(e, parent2)
		}
		if err != nil {
			return
		}

		unixTime := uint64(commitData.When.Unix())
		unixTime |= uint64(commitData.Generation) << 34
		if err = binary.WriteUint64(e, unixTime); err != nil {
			return
		}
	}
	return
}

func (e *Encoder) encodeExtraEdges(extraEdges []uint32) (err error) {
	for _, parent := range extraEdges {
		if err = binary.WriteUint32(e, parent); err != nil {
			return
		}
	}
	return
}

func (e *Encoder) encodeChecksum() error {
	_, err := e.Write(e.hash.Sum(nil)[:20])
	return err
}
