package gitbloom

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

func (e *Encoder) Encode(idx Index) error {
	var err error

	// Get all the hashes in the input index
	hashes := idx.Hashes()

	// Sort the inout and prepare helper structures we'll need for encoding
	fanout, totalBloomSize := e.prepare(idx, hashes)

	chunkSignatures := [][]byte{oidFanoutSignature, oidLookupSignature, bloomIndexesSignature, bloomDataSignature}
	chunkSizes := []uint64{4 * 256, uint64(len(hashes)) * 20, uint64(len(hashes)) * 4, totalBloomSize}

	if err = e.encodeFileHeader(len(chunkSignatures)); err != nil {
		return err
	}
	if err = e.encodeChunkHeaders(chunkSignatures, chunkSizes); err != nil {
		return err
	}
	if err = e.encodeFanout(fanout); err != nil {
		return err
	}
	if err = e.encodeOidLookup(hashes); err != nil {
		return err
	}
	if err := e.encodeBloomIndexes(idx, hashes); err != nil {
		return err
	}
	if err := e.encodeBloomData(idx, hashes); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	return e.encodeChecksum()
}

func (e *Encoder) prepare(idx Index, hashes []plumbing.Hash) (fanout []uint32, totalBloomSize uint64) {
	// Sort the hashes and build our index
	plumbing.HashesSort(hashes)
	fanout = make([]uint32, 256)
	for _, hash := range hashes {
		fanout[hash[0]]++
	}

	// Convert the fanout to cumulative values
	for i := 1; i <= 0xff; i++ {
		fanout[i] += fanout[i-1]
	}

	// Find out the total size of bloom filters
	for _, hash := range hashes {
		if bloom, _ := idx.GetBloomByHash(hash); bloom != nil {
			totalBloomSize += uint64(len(bloom.Data()) / 8)
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

func (e *Encoder) encodeBloomIndexes(idx Index, hashes []plumbing.Hash) (err error) {
	currentBloomSize := uint32(0)
	for _, hash := range hashes {
		if bloom, _ := idx.GetBloomByHash(hash); bloom != nil {
			currentBloomSize += uint32(len(bloom.Data()) / 8)
		}
		if err = binary.WriteUint32(e, currentBloomSize); err != nil {
			return
		}
	}
	return
}

func (e *Encoder) encodeBloomData(idx Index, hashes []plumbing.Hash) (err error) {
	if err = binary.WriteUint32(e, 1); err != nil {
		return
	}
	if err = binary.WriteUint32(e, 7); err != nil {
		return
	}
	if err = binary.WriteUint32(e, 10); err != nil {
		return
	}
	for _, hash := range hashes {
		if bloom, _ := idx.GetBloomByHash(hash); bloom != nil {
			if _, err = e.Write(bloom.Data()); err != nil {
				return err
			}
		}
	}
	return
}

func (e *Encoder) encodeChecksum() error {
	_, err := e.Write(e.hash.Sum(nil)[:20])
	return err
}
