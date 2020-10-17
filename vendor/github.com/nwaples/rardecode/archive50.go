package rardecode

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"io/ioutil"
	"time"
)

const (
	// block types
	block5Arc     = 1
	block5File    = 2
	block5Service = 3
	block5Encrypt = 4
	block5End     = 5

	// block flags
	block5HasExtra     = 0x0001
	block5HasData      = 0x0002
	block5DataNotFirst = 0x0008
	block5DataNotLast  = 0x0010

	// end block flags
	endArc5NotLast = 0x0001

	// archive encryption block flags
	enc5CheckPresent = 0x0001 // password check data is present

	// main archive block flags
	arc5MultiVol = 0x0001
	arc5Solid    = 0x0004

	// file block flags
	file5IsDir          = 0x0001
	file5HasUnixMtime   = 0x0002
	file5HasCRC32       = 0x0004
	file5UnpSizeUnknown = 0x0008

	// file encryption record flags
	file5EncCheckPresent = 0x0001 // password check data is present
	file5EncUseMac       = 0x0002 // use MAC instead of plain checksum

	cacheSize50   = 4
	maxPbkdf2Salt = 64
	pwCheckSize   = 8
	maxKdfCount   = 24

	minHeaderSize = 7
)

var (
	errBadPassword      = errors.New("rardecode: incorrect password")
	errCorruptEncrypt   = errors.New("rardecode: corrupt encryption data")
	errUnknownEncMethod = errors.New("rardecode: unknown encryption method")
)

type extra struct {
	ftype uint64  // field type
	data  readBuf // field data
}

type blockHeader50 struct {
	htype    uint64 // block type
	flags    uint64
	data     readBuf // block header data
	extra    []extra // extra fields
	dataSize int64   // size of block data
}

// leHash32 wraps a hash.Hash32 to return the result of Sum in little
// endian format.
type leHash32 struct {
	hash.Hash32
}

func (h leHash32) Sum(b []byte) []byte {
	s := h.Sum32()
	return append(b, byte(s), byte(s>>8), byte(s>>16), byte(s>>24))
}

func newLittleEndianCRC32() hash.Hash32 {
	return leHash32{crc32.NewIEEE()}
}

// hash50 implements fileChecksum for RAR 5 archives
type hash50 struct {
	hash.Hash        // hash file data is written to
	sum       []byte // file checksum
	key       []byte // if present used with hmac in calculating checksum from hash
}

func (h *hash50) valid() bool {
	sum := h.Sum(nil)
	if len(h.key) > 0 {
		mac := hmac.New(sha256.New, h.key)
		mac.Write(sum)
		sum = mac.Sum(sum[:0])
		if len(h.sum) == 4 {
			// CRC32
			for i, v := range sum[4:] {
				sum[i&3] ^= v
			}
			sum = sum[:4]
		}
	}
	return bytes.Equal(sum, h.sum)
}

// archive50 implements fileBlockReader for RAR 5 file format archives
type archive50 struct {
	byteReader               // reader for current block data
	v          *bufio.Reader // reader for current archive volume
	pass       []byte
	blockKey   []byte                // key used to encrypt blocks
	multi      bool                  // archive is multi-volume
	solid      bool                  // is a solid archive
	checksum   hash50                // file checksum
	dec        decoder               // optional decoder used to unpack file
	buf        readBuf               // temporary buffer
	keyCache   [cacheSize50]struct { // encryption key cache
		kdfCount int
		salt     []byte
		keys     [][]byte
	}
}

// calcKeys50 calculates the keys used in RAR 5 archive processing.
// The returned slice of byte slices contains 3 keys.
// Key 0 is used for block or file decryption.
// Key 1 is optionally used for file checksum calculation.
// Key 2 is optionally used for password checking.
func calcKeys50(pass, salt []byte, kdfCount int) [][]byte {
	if len(salt) > maxPbkdf2Salt {
		salt = salt[:maxPbkdf2Salt]
	}
	keys := make([][]byte, 3)
	if len(keys) == 0 {
		return keys
	}

	prf := hmac.New(sha256.New, pass)
	prf.Write(salt)
	prf.Write([]byte{0, 0, 0, 1})

	t := prf.Sum(nil)
	u := append([]byte(nil), t...)

	kdfCount--

	for i, iter := range []int{kdfCount, 16, 16} {
		for iter > 0 {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for j := range u {
				t[j] ^= u[j]
			}
			iter--
		}
		keys[i] = append([]byte(nil), t...)
	}

	pwcheck := keys[2]
	for i, v := range pwcheck[pwCheckSize:] {
		pwcheck[i&(pwCheckSize-1)] ^= v
	}
	keys[2] = pwcheck[:pwCheckSize]

	return keys
}

// getKeys reads kdfcount and salt from b and returns the corresponding encryption keys.
func (a *archive50) getKeys(b *readBuf) (keys [][]byte, err error) {
	if len(*b) < 17 {
		return nil, errCorruptEncrypt
	}
	// read kdf count and salt
	kdfCount := int(b.byte())
	if kdfCount > maxKdfCount {
		return nil, errCorruptEncrypt
	}
	kdfCount = 1 << uint(kdfCount)
	salt := b.bytes(16)

	// check cache of keys for match
	for _, v := range a.keyCache {
		if kdfCount == v.kdfCount && bytes.Equal(salt, v.salt) {
			return v.keys, nil
		}
	}
	// not found, calculate keys
	keys = calcKeys50(a.pass, salt, kdfCount)

	// store in cache
	copy(a.keyCache[1:], a.keyCache[:])
	a.keyCache[0].kdfCount = kdfCount
	a.keyCache[0].salt = append([]byte(nil), salt...)
	a.keyCache[0].keys = keys

	return keys, nil
}

// checkPassword calculates if a password is correct given password check data and keys.
func checkPassword(b *readBuf, keys [][]byte) error {
	if len(*b) < 12 {
		return nil // not enough bytes, ignore for the moment
	}
	pwcheck := b.bytes(8)
	sum := b.bytes(4)
	csum := sha256.Sum256(pwcheck)
	if bytes.Equal(sum, csum[:len(sum)]) && !bytes.Equal(pwcheck, keys[2]) {
		return errBadPassword
	}
	return nil
}

// parseFileEncryptionRecord processes the optional file encryption record from a file header.
func (a *archive50) parseFileEncryptionRecord(b readBuf, f *fileBlockHeader) error {
	if ver := b.uvarint(); ver != 0 {
		return errUnknownEncMethod
	}
	flags := b.uvarint()

	keys, err := a.getKeys(&b)
	if err != nil {
		return err
	}

	f.key = keys[0]
	if len(b) < 16 {
		return errCorruptEncrypt
	}
	f.iv = b.bytes(16)

	if flags&file5EncCheckPresent > 0 {
		if err := checkPassword(&b, keys); err != nil {
			return err
		}
	}
	if flags&file5EncUseMac > 0 {
		a.checksum.key = keys[1]
	}
	return nil
}

func (a *archive50) parseFileHeader(h *blockHeader50) (*fileBlockHeader, error) {
	a.checksum.sum = nil
	a.checksum.key = nil

	f := new(fileBlockHeader)

	f.first = h.flags&block5DataNotFirst == 0
	f.last = h.flags&block5DataNotLast == 0

	flags := h.data.uvarint() // file flags
	f.IsDir = flags&file5IsDir > 0
	f.UnKnownSize = flags&file5UnpSizeUnknown > 0
	f.UnPackedSize = int64(h.data.uvarint())
	f.PackedSize = h.dataSize
	f.Attributes = int64(h.data.uvarint())
	if flags&file5HasUnixMtime > 0 {
		if len(h.data) < 4 {
			return nil, errCorruptFileHeader
		}
		f.ModificationTime = time.Unix(int64(h.data.uint32()), 0)
	}
	if flags&file5HasCRC32 > 0 {
		if len(h.data) < 4 {
			return nil, errCorruptFileHeader
		}
		a.checksum.sum = append([]byte(nil), h.data.bytes(4)...)
		if f.first {
			a.checksum.Hash = newLittleEndianCRC32()
			f.cksum = &a.checksum
		}
	}

	flags = h.data.uvarint() // compression flags
	f.solid = flags&0x0040 > 0
	f.winSize = uint(flags&0x3C00)>>10 + 17
	method := (flags >> 7) & 7 // compression method (0 == none)
	if f.first && method != 0 {
		unpackver := flags & 0x003f
		if unpackver != 0 {
			return nil, errUnknownDecoder
		}
		if a.dec == nil {
			a.dec = new(decoder50)
		}
		f.decoder = a.dec
	}
	switch h.data.uvarint() {
	case 0:
		f.HostOS = HostOSWindows
	case 1:
		f.HostOS = HostOSUnix
	default:
		f.HostOS = HostOSUnknown
	}
	nlen := int(h.data.uvarint())
	if len(h.data) < nlen {
		return nil, errCorruptFileHeader
	}
	f.Name = string(h.data.bytes(nlen))

	// parse optional extra records
	for _, e := range h.extra {
		var err error
		switch e.ftype {
		case 1: // encryption
			err = a.parseFileEncryptionRecord(e.data, f)
		case 2:
			// TODO: hash
		case 3:
			// TODO: time
		case 4: // version
			_ = e.data.uvarint() // ignore flags field
			f.Version = int(e.data.uvarint())
		case 5:
			// TODO: redirection
		case 6:
			// TODO: owner
		}
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

// parseEncryptionBlock calculates the key for block encryption.
func (a *archive50) parseEncryptionBlock(b readBuf) error {
	if ver := b.uvarint(); ver != 0 {
		return errUnknownEncMethod
	}
	flags := b.uvarint()
	keys, err := a.getKeys(&b)
	if err != nil {
		return err
	}
	if flags&enc5CheckPresent > 0 {
		if err := checkPassword(&b, keys); err != nil {
			return err
		}
	}
	a.blockKey = keys[0]
	return nil
}

func (a *archive50) readBlockHeader() (*blockHeader50, error) {
	r := io.Reader(a.v)
	if a.blockKey != nil {
		// block is encrypted
		iv := a.buf[:16]
		if err := readFull(r, iv); err != nil {
			return nil, err
		}
		r = newAesDecryptReader(r, a.blockKey, iv)
	}

	b := a.buf[:minHeaderSize]
	if err := readFull(r, b); err != nil {
		return nil, err
	}
	crc := b.uint32()

	hash := crc32.NewIEEE()
	hash.Write(b)

	size := int(b.uvarint()) // header size
	if size > cap(a.buf) {
		a.buf = readBuf(make([]byte, size))
	} else {
		a.buf = a.buf[:size]
	}
	n := copy(a.buf, b)                            // copy left over bytes
	if err := readFull(r, a.buf[n:]); err != nil { // read rest of header
		return nil, err
	}

	// check header crc
	hash.Write(a.buf[n:])
	if crc != hash.Sum32() {
		return nil, errBadHeaderCrc
	}

	b = a.buf
	h := new(blockHeader50)
	h.htype = b.uvarint()
	h.flags = b.uvarint()

	var extraSize int
	if h.flags&block5HasExtra > 0 {
		extraSize = int(b.uvarint())
	}
	if h.flags&block5HasData > 0 {
		h.dataSize = int64(b.uvarint())
	}
	if len(b) < extraSize {
		return nil, errCorruptHeader
	}
	h.data = b.bytes(len(b) - extraSize)

	// read header extra records
	for len(b) > 0 {
		size = int(b.uvarint())
		if len(b) < size {
			return nil, errCorruptHeader
		}
		data := readBuf(b.bytes(size))
		ftype := data.uvarint()
		h.extra = append(h.extra, extra{ftype, data})
	}

	return h, nil
}

// next advances to the next file block in the archive
func (a *archive50) next() (*fileBlockHeader, error) {
	for {
		h, err := a.readBlockHeader()
		if err != nil {
			return nil, err
		}
		a.byteReader = limitByteReader(a.v, h.dataSize)
		switch h.htype {
		case block5File:
			return a.parseFileHeader(h)
		case block5Arc:
			flags := h.data.uvarint()
			a.multi = flags&arc5MultiVol > 0
			a.solid = flags&arc5Solid > 0
		case block5Encrypt:
			err = a.parseEncryptionBlock(h.data)
		case block5End:
			flags := h.data.uvarint()
			if flags&endArc5NotLast == 0 || !a.multi {
				return nil, errArchiveEnd
			}
			return nil, errArchiveContinues
		default:
			// discard block data
			_, err = io.Copy(ioutil.Discard, a.byteReader)
		}
		if err != nil {
			return nil, err
		}
	}
}

func (a *archive50) version() int { return fileFmt50 }

func (a *archive50) reset() {
	a.blockKey = nil // reset encryption when opening new volume file
}

func (a *archive50) isSolid() bool {
	return a.solid
}

// newArchive50 creates a new fileBlockReader for a Version 5 archive.
func newArchive50(r *bufio.Reader, password string) fileBlockReader {
	a := new(archive50)
	a.v = r
	a.pass = []byte(password)
	a.buf = make([]byte, 100)
	return a
}
