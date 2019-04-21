package gitbloom

import (
	"bytes"
	encbin "encoding/binary"
	"errors"
	"io"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/utils/binary"
)

var (
	// ErrUnsupportedVersion is returned by OpenFileIndex when the bloom filter
	// file version is not supported.
	ErrUnsupportedVersion = errors.New("Unsupported version")
	// ErrMalformedBloomFilterFile is returned by OpenFileIndex when the bloom
	// filter file is corrupted.
	ErrMalformedBloomFilterFile = errors.New("Malformed bloom filter file")

	commitFileSignature   = []byte{'C', 'G', 'P', 'H'}
	oidFanoutSignature    = []byte{'O', 'I', 'D', 'F'}
	oidLookupSignature    = []byte{'O', 'I', 'D', 'L'}
	bloomIndexesSignature = []byte{'B', 'I', 'D', 'X'}
	bloomDataSignature    = []byte{'B', 'D', 'A', 'T'}
	lastSignature         = []byte{0, 0, 0, 0}
)

type fileIndex struct {
	reader             io.ReaderAt
	fanout             [256]int
	oidFanoutOffset    int64
	oidLookupOffset    int64
	bloomIndexesOffset int64
	bloomDataOffset    int64
}

// OpenFileIndex opens a serialized commit graph file in the format described at
// https://github.com/git/git/blob/master/Documentation/technical/commit-graph-format.txt
func OpenFileIndex(reader io.ReaderAt) (Index, error) {
	fi := &fileIndex{reader: reader}

	if err := fi.verifyFileHeader(); err != nil {
		return nil, err
	}
	if err := fi.readChunkHeaders(); err != nil {
		return nil, err
	}
	if err := fi.readFanout(); err != nil {
		return nil, err
	}

	return fi, nil
}

func (fi *fileIndex) verifyFileHeader() error {
	// Verify file signature
	var signature = make([]byte, 4)
	if _, err := fi.reader.ReadAt(signature, 0); err != nil {
		return err
	}
	if !bytes.Equal(signature, commitFileSignature) {
		return ErrMalformedBloomFilterFile
	}

	// Read and verify the file header
	var header = make([]byte, 4)
	if _, err := fi.reader.ReadAt(header, 4); err != nil {
		return err
	}
	if header[0] != 1 {
		return ErrUnsupportedVersion
	}

	return nil
}

func (fi *fileIndex) readChunkHeaders() error {
	var chunkID = make([]byte, 4)
	for i := 0; ; i++ {
		chunkHeader := io.NewSectionReader(fi.reader, 8+(int64(i)*12), 12)
		if _, err := io.ReadAtLeast(chunkHeader, chunkID, 4); err != nil {
			return err
		}
		chunkOffset, err := binary.ReadUint64(chunkHeader)
		if err != nil {
			return err
		}

		if bytes.Equal(chunkID, oidFanoutSignature) {
			fi.oidFanoutOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, oidLookupSignature) {
			fi.oidLookupOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, bloomIndexesSignature) {
			fi.bloomIndexesOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, bloomDataSignature) {
			fi.bloomDataOffset = int64(chunkOffset)
		} else if bytes.Equal(chunkID, lastSignature) {
			break
		}
	}

	if fi.oidFanoutOffset <= 0 || fi.oidLookupOffset <= 0 || fi.bloomIndexesOffset <= 0 || fi.bloomDataOffset <= 0 {
		return ErrMalformedBloomFilterFile
	}

	return nil
}

func (fi *fileIndex) readFanout() error {
	fanoutReader := io.NewSectionReader(fi.reader, fi.oidFanoutOffset, 256*4)
	for i := 0; i < 256; i++ {
		fanoutValue, err := binary.ReadUint32(fanoutReader)
		if err != nil {
			return err
		}
		if fanoutValue > 0x7fffffff {
			return ErrMalformedBloomFilterFile
		}
		fi.fanout[i] = int(fanoutValue)
	}
	return nil
}

func (fi *fileIndex) getIndexByHash(h plumbing.Hash) (int, error) {
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

func (fi *fileIndex) GetBloomByHash(h plumbing.Hash) (*BloomPathFilter, error) {
	idx, err := fi.getIndexByHash(h)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 4)
	prevIndex := uint32(0)
	if idx > 0 {
		if _, err := fi.reader.ReadAt(buf, fi.bloomIndexesOffset+int64(idx-1)*4); err != nil {
			return nil, err
		}
		prevIndex = encbin.BigEndian.Uint32(buf)
	}
	if _, err := fi.reader.ReadAt(buf, fi.bloomIndexesOffset+int64(idx)*4); err != nil {
		return nil, err
	}
	nextIndex := encbin.BigEndian.Uint32(buf)

	length := nextIndex - prevIndex
	if length == 0 {
		return nil, plumbing.ErrObjectNotFound
	}
	data := make([]byte, length*8)
	_, err = fi.reader.ReadAt(data, fi.bloomDataOffset+12+int64(prevIndex)*8)
	if err != nil {
		return nil, err
	}

	return LoadBloomPathFilter(data), nil
}

func (fi *fileIndex) Hashes() []plumbing.Hash {
	hashes := make([]plumbing.Hash, fi.fanout[0xff])
	for i := 0; i < int(fi.fanout[0xff]); i++ {
		offset := fi.oidLookupOffset + int64(i)*20
		if n, err := fi.reader.ReadAt(hashes[i][:], offset); err != nil || n < 20 {
			return nil
		}
	}
	return hashes
}
