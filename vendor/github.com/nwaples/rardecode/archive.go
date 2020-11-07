package rardecode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	maxSfxSize = 0x100000 // maximum number of bytes to read when searching for RAR signature
	sigPrefix  = "Rar!\x1A\x07"

	fileFmt15 = iota + 1 // Version 1.5 archive file format
	fileFmt50            // Version 5.0 archive file format
)

var (
	errNoSig              = errors.New("rardecode: RAR signature not found")
	errVerMismatch        = errors.New("rardecode: volume version mistmatch")
	errCorruptHeader      = errors.New("rardecode: corrupt block header")
	errCorruptFileHeader  = errors.New("rardecode: corrupt file header")
	errBadHeaderCrc       = errors.New("rardecode: bad header crc")
	errUnknownArc         = errors.New("rardecode: unknown archive version")
	errUnknownDecoder     = errors.New("rardecode: unknown decoder version")
	errUnsupportedDecoder = errors.New("rardecode: unsupported decoder version")
	errArchiveContinues   = errors.New("rardecode: archive continues in next volume")
	errArchiveEnd         = errors.New("rardecode: archive end reached")
	errDecoderOutOfData   = errors.New("rardecode: decoder expected more data than is in packed file")

	reDigits = regexp.MustCompile(`\d+`)
)

type readBuf []byte

func (b *readBuf) byte() byte {
	v := (*b)[0]
	*b = (*b)[1:]
	return v
}

func (b *readBuf) uint16() uint16 {
	v := uint16((*b)[0]) | uint16((*b)[1])<<8
	*b = (*b)[2:]
	return v
}

func (b *readBuf) uint32() uint32 {
	v := uint32((*b)[0]) | uint32((*b)[1])<<8 | uint32((*b)[2])<<16 | uint32((*b)[3])<<24
	*b = (*b)[4:]
	return v
}

func (b *readBuf) bytes(n int) []byte {
	v := (*b)[:n]
	*b = (*b)[n:]
	return v
}

func (b *readBuf) uvarint() uint64 {
	var x uint64
	var s uint
	for i, n := range *b {
		if n < 0x80 {
			*b = (*b)[i+1:]
			return x | uint64(n)<<s
		}
		x |= uint64(n&0x7f) << s
		s += 7

	}
	// if we run out of bytes, just return 0
	*b = (*b)[len(*b):]
	return 0
}

// readFull wraps io.ReadFull to return io.ErrUnexpectedEOF instead
// of io.EOF when 0 bytes are read.
func readFull(r io.Reader, buf []byte) error {
	_, err := io.ReadFull(r, buf)
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// findSig searches for the RAR signature and version at the beginning of a file.
// It searches no more than maxSfxSize bytes.
func findSig(br *bufio.Reader) (int, error) {
	for n := 0; n <= maxSfxSize; {
		b, err := br.ReadSlice(sigPrefix[0])
		n += len(b)
		if err == bufio.ErrBufferFull {
			continue
		} else if err != nil {
			if err == io.EOF {
				err = errNoSig
			}
			return 0, err
		}

		b, err = br.Peek(len(sigPrefix[1:]) + 2)
		if err != nil {
			if err == io.EOF {
				err = errNoSig
			}
			return 0, err
		}
		if !bytes.HasPrefix(b, []byte(sigPrefix[1:])) {
			continue
		}
		b = b[len(sigPrefix)-1:]

		var ver int
		switch {
		case b[0] == 0:
			ver = fileFmt15
		case b[0] == 1 && b[1] == 0:
			ver = fileFmt50
		default:
			continue
		}
		_, _ = br.ReadSlice('\x00')

		return ver, nil
	}
	return 0, errNoSig
}

// volume extends a fileBlockReader to be used across multiple
// files in a multi-volume archive
type volume struct {
	fileBlockReader
	f     *os.File      // current file handle
	br    *bufio.Reader // buffered reader for current volume file
	dir   string        // volume directory
	file  string        // current volume file (not including directory)
	files []string      // full path names for current volume files processed
	num   int           // volume number
	old   bool          // uses old naming scheme
}

// nextVolName updates name to the next filename in the archive.
func (v *volume) nextVolName() {
	if v.num == 0 {
		// check file extensions
		i := strings.LastIndex(v.file, ".")
		if i < 0 {
			// no file extension, add one
			i = len(v.file)
			v.file += ".rar"
		} else {
			ext := strings.ToLower(v.file[i+1:])
			// replace with .rar for empty extensions & self extracting archives
			if ext == "" || ext == "exe" || ext == "sfx" {
				v.file = v.file[:i+1] + "rar"
			}
		}
		if a, ok := v.fileBlockReader.(*archive15); ok {
			v.old = a.old
		}
		// new naming scheme must have volume number in filename
		if !v.old && reDigits.FindStringIndex(v.file) == nil {
			v.old = true
		}
		// For old style naming if 2nd and 3rd character of file extension is not a digit replace
		// with "00" and ignore any trailing characters.
		if v.old && (len(v.file) < i+4 || v.file[i+2] < '0' || v.file[i+2] > '9' || v.file[i+3] < '0' || v.file[i+3] > '9') {
			v.file = v.file[:i+2] + "00"
			return
		}
	}
	// new style volume naming
	if !v.old {
		// find all numbers in volume name
		m := reDigits.FindAllStringIndex(v.file, -1)
		if l := len(m); l > 1 {
			// More than 1 match so assume name.part###of###.rar style.
			// Take the last 2 matches where the first is the volume number.
			m = m[l-2 : l]
			if strings.Contains(v.file[m[0][1]:m[1][0]], ".") || !strings.Contains(v.file[:m[0][0]], ".") {
				// Didn't match above style as volume had '.' between the two numbers or didnt have a '.'
				// before the first match. Use the second number as volume number.
				m = m[1:]
			}
		}
		// extract and increment volume number
		lo, hi := m[0][0], m[0][1]
		n, err := strconv.Atoi(v.file[lo:hi])
		if err != nil {
			n = 0
		} else {
			n++
		}
		// volume number must use at least the same number of characters as previous volume
		vol := fmt.Sprintf("%0"+fmt.Sprint(hi-lo)+"d", n)
		v.file = v.file[:lo] + vol + v.file[hi:]
		return
	}
	// old style volume naming
	i := strings.LastIndex(v.file, ".")
	// get file extension
	b := []byte(v.file[i+1:])
	// start incrementing volume number digits from rightmost
	for j := 2; j >= 0; j-- {
		if b[j] != '9' {
			b[j]++
			break
		}
		// digit overflow
		if j == 0 {
			// last character before '.'
			b[j] = 'A'
		} else {
			// set to '0' and loop to next character
			b[j] = '0'
		}
	}
	v.file = v.file[:i+1] + string(b)
}

func (v *volume) next() (*fileBlockHeader, error) {
	for {
		var atEOF bool

		h, err := v.fileBlockReader.next()
		switch err {
		case errArchiveContinues:
		case io.EOF:
			// Read all of volume without finding an end block. The only way
			// to tell if the archive continues is to try to open the next volume.
			atEOF = true
		default:
			return h, err
		}

		v.f.Close()
		v.nextVolName()
		v.f, err = os.Open(v.dir + v.file) // Open next volume file
		if err != nil {
			if atEOF && os.IsNotExist(err) {
				// volume not found so assume that the archive has ended
				return nil, io.EOF
			}
			return nil, err
		}
		v.num++
		v.br.Reset(v.f)
		ver, err := findSig(v.br)
		if err != nil {
			return nil, err
		}
		if v.version() != ver {
			return nil, errVerMismatch
		}
		v.files = append(v.files, v.dir+v.file)
		v.reset() // reset encryption
	}
}

func (v *volume) Close() error {
	// may be nil if os.Open fails in next()
	if v.f == nil {
		return nil
	}
	return v.f.Close()
}

func openVolume(name, password string) (*volume, error) {
	var err error
	v := new(volume)
	v.dir, v.file = filepath.Split(name)
	v.f, err = os.Open(name)
	if err != nil {
		return nil, err
	}
	v.br = bufio.NewReader(v.f)
	v.fileBlockReader, err = newFileBlockReader(v.br, password)
	if err != nil {
		v.f.Close()
		return nil, err
	}
	v.files = append(v.files, name)
	return v, nil
}

func newFileBlockReader(br *bufio.Reader, pass string) (fileBlockReader, error) {
	runes := []rune(pass)
	if len(runes) > maxPassword {
		pass = string(runes[:maxPassword])
	}
	ver, err := findSig(br)
	if err != nil {
		return nil, err
	}
	switch ver {
	case fileFmt15:
		return newArchive15(br, pass), nil
	case fileFmt50:
		return newArchive50(br, pass), nil
	}
	return nil, errUnknownArc
}
