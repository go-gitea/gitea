package rardecode

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	// block types
	blockArc     = 0x73
	blockFile    = 0x74
	blockService = 0x7a
	blockEnd     = 0x7b

	// block flags
	blockHasData = 0x8000

	// archive block flags
	arcVolume    = 0x0001
	arcSolid     = 0x0008
	arcNewNaming = 0x0010
	arcEncrypted = 0x0080

	// file block flags
	fileSplitBefore = 0x0001
	fileSplitAfter  = 0x0002
	fileEncrypted   = 0x0004
	fileSolid       = 0x0010
	fileWindowMask  = 0x00e0
	fileLargeData   = 0x0100
	fileUnicode     = 0x0200
	fileSalt        = 0x0400
	fileVersion     = 0x0800
	fileExtTime     = 0x1000

	// end block flags
	endArcNotLast = 0x0001

	saltSize    = 8 // size of salt for calculating AES keys
	cacheSize30 = 4 // number of AES keys to cache
	hashRounds  = 0x40000
)

var (
	errMultipleDecoders = errors.New("rardecode: multiple decoders in a single archive not supported")
)

type blockHeader15 struct {
	htype    byte // block header type
	flags    uint16
	data     readBuf // header data
	dataSize int64   // size of extra block data
}

// fileHash32 implements fileChecksum for 32-bit hashes
type fileHash32 struct {
	hash.Hash32        // hash to write file contents to
	sum         uint32 // 32bit checksum for file
}

func (h *fileHash32) valid() bool {
	return h.sum == h.Sum32()
}

// archive15 implements fileBlockReader for RAR 1.5 file format archives
type archive15 struct {
	byteReader               // reader for current block data
	v          *bufio.Reader // reader for current archive volume
	dec        decoder       // current decoder
	decVer     byte          // current decoder version
	multi      bool          // archive is multi-volume
	old        bool          // archive uses old naming scheme
	solid      bool          // archive is a solid archive
	encrypted  bool
	pass       []uint16              // password in UTF-16
	checksum   fileHash32            // file checksum
	buf        readBuf               // temporary buffer
	keyCache   [cacheSize30]struct { // cache of previously calculated decryption keys
		salt []byte
		key  []byte
		iv   []byte
	}
}

// Calculates the key and iv for AES decryption given a password and salt.
func calcAes30Params(pass []uint16, salt []byte) (key, iv []byte) {
	p := make([]byte, 0, len(pass)*2+len(salt))
	for _, v := range pass {
		p = append(p, byte(v), byte(v>>8))
	}
	p = append(p, salt...)

	hash := sha1.New()
	iv = make([]byte, 16)
	s := make([]byte, 0, hash.Size())
	for i := 0; i < hashRounds; i++ {
		hash.Write(p)
		hash.Write([]byte{byte(i), byte(i >> 8), byte(i >> 16)})
		if i%(hashRounds/16) == 0 {
			s = hash.Sum(s[:0])
			iv[i/(hashRounds/16)] = s[4*4+3]
		}
	}
	key = hash.Sum(s[:0])
	key = key[:16]

	for k := key; len(k) >= 4; k = k[4:] {
		k[0], k[1], k[2], k[3] = k[3], k[2], k[1], k[0]
	}
	return key, iv
}

// parseDosTime converts a 32bit DOS time value to time.Time
func parseDosTime(t uint32) time.Time {
	n := int(t)
	sec := n & 0x1f << 1
	min := n >> 5 & 0x3f
	hr := n >> 11 & 0x1f
	day := n >> 16 & 0x1f
	mon := time.Month(n >> 21 & 0x0f)
	yr := n>>25&0x7f + 1980
	return time.Date(yr, mon, day, hr, min, sec, 0, time.Local)
}

// decodeName decodes a non-unicode filename from a file header.
func decodeName(buf []byte) string {
	i := bytes.IndexByte(buf, 0)
	if i < 0 {
		return string(buf) // filename is UTF-8
	}

	name := buf[:i]
	encName := readBuf(buf[i+1:])
	if len(encName) < 2 {
		return "" // invalid encoding
	}
	highByte := uint16(encName.byte()) << 8
	flags := encName.byte()
	flagBits := 8
	var wchars []uint16 // decoded characters are UTF-16
	for len(wchars) < len(name) && len(encName) > 0 {
		if flagBits == 0 {
			flags = encName.byte()
			flagBits = 8
			if len(encName) == 0 {
				break
			}
		}
		switch flags >> 6 {
		case 0:
			wchars = append(wchars, uint16(encName.byte()))
		case 1:
			wchars = append(wchars, uint16(encName.byte())|highByte)
		case 2:
			if len(encName) < 2 {
				break
			}
			wchars = append(wchars, encName.uint16())
		case 3:
			n := encName.byte()
			b := name[len(wchars):]
			if l := int(n&0x7f) + 2; l < len(b) {
				b = b[:l]
			}
			if n&0x80 > 0 {
				if len(encName) < 1 {
					break
				}
				ec := encName.byte()
				for _, c := range b {
					wchars = append(wchars, uint16(c+ec)|highByte)
				}
			} else {
				for _, c := range b {
					wchars = append(wchars, uint16(c))
				}
			}
		}
		flags <<= 2
		flagBits -= 2
	}
	return string(utf16.Decode(wchars))
}

// readExtTimes reads and parses the optional extra time field from the file header.
func readExtTimes(f *fileBlockHeader, b *readBuf) {
	if len(*b) < 2 {
		return // invalid, not enough data
	}
	flags := b.uint16()

	ts := []*time.Time{&f.ModificationTime, &f.CreationTime, &f.AccessTime}

	for i, t := range ts {
		n := flags >> uint((3-i)*4)
		if n&0x8 == 0 {
			continue
		}
		if i != 0 { // ModificationTime already read so skip
			if len(*b) < 4 {
				return // invalid, not enough data
			}
			*t = parseDosTime(b.uint32())
		}
		if n&0x4 > 0 {
			*t = t.Add(time.Second)
		}
		n &= 0x3
		if n == 0 {
			continue
		}
		if len(*b) < int(n) {
			return // invalid, not enough data
		}
		// add extra time data in 100's of nanoseconds
		d := time.Duration(0)
		for j := 3 - n; j < n; j++ {
			d |= time.Duration(b.byte()) << (j * 8)
		}
		d *= 100
		*t = t.Add(d)
	}
}

func (a *archive15) getKeys(salt []byte) (key, iv []byte) {
	// check cache of keys
	for _, v := range a.keyCache {
		if bytes.Equal(v.salt[:], salt) {
			return v.key, v.iv
		}
	}
	key, iv = calcAes30Params(a.pass, salt)

	// save a copy in the cache
	copy(a.keyCache[1:], a.keyCache[:])
	a.keyCache[0].salt = append([]byte(nil), salt...) // copy so byte slice can be reused
	a.keyCache[0].key = key
	a.keyCache[0].iv = iv

	return key, iv
}

func (a *archive15) parseFileHeader(h *blockHeader15) (*fileBlockHeader, error) {
	f := new(fileBlockHeader)

	f.first = h.flags&fileSplitBefore == 0
	f.last = h.flags&fileSplitAfter == 0

	f.solid = h.flags&fileSolid > 0
	f.IsDir = h.flags&fileWindowMask == fileWindowMask
	if !f.IsDir {
		f.winSize = uint(h.flags&fileWindowMask)>>5 + 16
	}

	b := h.data
	if len(b) < 21 {
		return nil, errCorruptFileHeader
	}

	f.PackedSize = h.dataSize
	f.UnPackedSize = int64(b.uint32())
	f.HostOS = b.byte() + 1
	if f.HostOS > HostOSBeOS {
		f.HostOS = HostOSUnknown
	}
	a.checksum.sum = b.uint32()

	f.ModificationTime = parseDosTime(b.uint32())
	unpackver := b.byte()     // decoder version
	method := b.byte() - 0x30 // decryption method
	namesize := int(b.uint16())
	f.Attributes = int64(b.uint32())
	if h.flags&fileLargeData > 0 {
		if len(b) < 8 {
			return nil, errCorruptFileHeader
		}
		_ = b.uint32() // already read large PackedSize in readBlockHeader
		f.UnPackedSize |= int64(b.uint32()) << 32
		f.UnKnownSize = f.UnPackedSize == -1
	} else if int32(f.UnPackedSize) == -1 {
		f.UnKnownSize = true
		f.UnPackedSize = -1
	}
	if len(b) < namesize {
		return nil, errCorruptFileHeader
	}
	name := b.bytes(namesize)
	if h.flags&fileUnicode == 0 {
		f.Name = string(name)
	} else {
		f.Name = decodeName(name)
	}
	// Rar 4.x uses '\' as file separator
	f.Name = strings.Replace(f.Name, "\\", "/", -1)

	if h.flags&fileVersion > 0 {
		// file version is stored as ';n' appended to file name
		i := strings.LastIndex(f.Name, ";")
		if i > 0 {
			j, err := strconv.Atoi(f.Name[i+1:])
			if err == nil && j >= 0 {
				f.Version = j
				f.Name = f.Name[:i]
			}
		}
	}

	var salt []byte
	if h.flags&fileSalt > 0 {
		if len(b) < saltSize {
			return nil, errCorruptFileHeader
		}
		salt = b.bytes(saltSize)
	}
	if h.flags&fileExtTime > 0 {
		readExtTimes(f, &b)
	}

	if !f.first {
		return f, nil
	}
	// fields only needed for first block in a file
	if h.flags&fileEncrypted > 0 && len(salt) == saltSize {
		f.key, f.iv = a.getKeys(salt)
	}
	a.checksum.Reset()
	f.cksum = &a.checksum
	if method == 0 {
		return f, nil
	}
	if a.dec == nil {
		switch unpackver {
		case 15, 20, 26:
			return nil, errUnsupportedDecoder
		case 29:
			a.dec = new(decoder29)
		default:
			return nil, errUnknownDecoder
		}
		a.decVer = unpackver
	} else if a.decVer != unpackver {
		return nil, errMultipleDecoders
	}
	f.decoder = a.dec
	return f, nil
}

// readBlockHeader returns the next block header in the archive.
// It will return io.EOF if there were no bytes read.
func (a *archive15) readBlockHeader() (*blockHeader15, error) {
	var err error
	b := a.buf[:7]
	r := io.Reader(a.v)
	if a.encrypted {
		salt := a.buf[:saltSize]
		_, err = io.ReadFull(r, salt)
		if err != nil {
			return nil, err
		}
		key, iv := a.getKeys(salt)
		r = newAesDecryptReader(r, key, iv)
		err = readFull(r, b)
	} else {
		_, err = io.ReadFull(r, b)
	}
	if err != nil {
		return nil, err
	}

	crc := b.uint16()
	hash := crc32.NewIEEE()
	hash.Write(b)
	h := new(blockHeader15)
	h.htype = b.byte()
	h.flags = b.uint16()
	size := b.uint16()
	if size < 7 {
		return nil, errCorruptHeader
	}
	size -= 7
	if int(size) > cap(a.buf) {
		a.buf = readBuf(make([]byte, size))
	}
	h.data = a.buf[:size]
	if err := readFull(r, h.data); err != nil {
		return nil, err
	}
	hash.Write(h.data)
	if crc != uint16(hash.Sum32()) {
		return nil, errBadHeaderCrc
	}
	if h.flags&blockHasData > 0 {
		if len(h.data) < 4 {
			return nil, errCorruptHeader
		}
		h.dataSize = int64(h.data.uint32())
	}
	if (h.htype == blockService || h.htype == blockFile) && h.flags&fileLargeData > 0 {
		if len(h.data) < 25 {
			return nil, errCorruptHeader
		}
		b := h.data[21:25]
		h.dataSize |= int64(b.uint32()) << 32
	}
	return h, nil
}

// next advances to the next file block in the archive
func (a *archive15) next() (*fileBlockHeader, error) {
	for {
		// could return an io.EOF here as 1.5 archives may not have an end block.
		h, err := a.readBlockHeader()
		if err != nil {
			return nil, err
		}
		a.byteReader = limitByteReader(a.v, h.dataSize) // reader for block data

		switch h.htype {
		case blockFile:
			return a.parseFileHeader(h)
		case blockArc:
			a.encrypted = h.flags&arcEncrypted > 0
			a.multi = h.flags&arcVolume > 0
			a.old = h.flags&arcNewNaming == 0
			a.solid = h.flags&arcSolid > 0
		case blockEnd:
			if h.flags&endArcNotLast == 0 || !a.multi {
				return nil, errArchiveEnd
			}
			return nil, errArchiveContinues
		default:
			_, err = io.Copy(ioutil.Discard, a.byteReader)
		}
		if err != nil {
			return nil, err
		}
	}
}

func (a *archive15) version() int { return fileFmt15 }

func (a *archive15) reset() {
	a.encrypted = false // reset encryption when opening new volume file
}

func (a *archive15) isSolid() bool {
	return a.solid
}

// newArchive15 creates a new fileBlockReader for a Version 1.5 archive
func newArchive15(r *bufio.Reader, password string) fileBlockReader {
	a := new(archive15)
	a.v = r
	a.pass = utf16.Encode([]rune(password)) // convert to UTF-16
	a.checksum.Hash32 = crc32.NewIEEE()
	a.buf = readBuf(make([]byte, 100))
	return a
}
