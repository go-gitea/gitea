// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package armor implements OpenPGP ASCII Armor, see RFC 4880. OpenPGP Armor is
// very similar to PEM except that it has an additional CRC checksum.
package armor // import "github.com/keybase/go-crypto/openpgp/armor"

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/keybase/go-crypto/openpgp/errors"
)

// A Block represents an OpenPGP armored structure.
//
// The encoded form is:
//    -----BEGIN Type-----
//    Headers
//
//    base64-encoded Bytes
//    '=' base64 encoded checksum
//    -----END Type-----
// where Headers is a possibly empty sequence of Key: Value lines.
//
// Since the armored data can be very large, this package presents a streaming
// interface.
type Block struct {
	Type    string            // The type, taken from the preamble (i.e. "PGP SIGNATURE").
	Header  map[string]string // Optional headers.
	Body    io.Reader         // A Reader from which the contents can be read
	lReader lineReader
	oReader openpgpReader
}

var ArmorCorrupt error = errors.StructuralError("armor invalid")

const crc24Init = 0xb704ce
const crc24Poly = 0x1864cfb
const crc24Mask = 0xffffff

// crc24 calculates the OpenPGP checksum as specified in RFC 4880, section 6.1
func crc24(crc uint32, d []byte) uint32 {
	for _, b := range d {
		crc ^= uint32(b) << 16
		for i := 0; i < 8; i++ {
			crc <<= 1
			if crc&0x1000000 != 0 {
				crc ^= crc24Poly
			}
		}
	}
	return crc
}

var armorStart = []byte("-----BEGIN ")
var armorEnd = []byte("-----END ")
var armorEndOfLine = []byte("-----")

// lineReader wraps a line based reader. It watches for the end of an armor
// block and records the expected CRC value.
type lineReader struct {
	in  *bufio.Reader
	buf []byte
	eof bool
	crc *uint32
}

// ourIsSpace checks if a rune is either space according to unicode
// package, or ZeroWidthSpace (which is not a space according to
// unicode module). Used to trim lines during header reading.
func ourIsSpace(r rune) bool {
	return r == '\u200b' || unicode.IsSpace(r)
}

func (l *lineReader) Read(p []byte) (n int, err error) {
	if l.eof {
		return 0, io.EOF
	}

	if len(l.buf) > 0 {
		n = copy(p, l.buf)
		l.buf = l.buf[n:]
		return
	}

	line, isPrefix, err := l.in.ReadLine()
	if err != nil {
		return
	}

	// Entry-level cleanup, just trim spaces.
	line = bytes.TrimFunc(line, ourIsSpace)

	lineWithChecksum := false
	foldedChecksum := false
	if !isPrefix && len(line) >= 5 && line[len(line)-5] == '=' && line[len(line)-4] != '=' {
		// This is the checksum line. Checksum should appear on separate line,
		// but some bundles don't have a newline between main payload and the
		// checksum, and we try to support that.

		// `=` is not a base64 character with the exception of padding, and the
		// padding can only be 2 characters long at most ("=="), so we can
		// safely assume that 5 characters starting with `=` at the end of the
		// line can't be a valid ending of a base64 stream. In other words, `=`
		// at position len-5 in base64 stream can never be a valid part of that
		// stream.

		// Checksum can never appear if isPrefix is true - that is, when
		// ReadLine returned non-final part of some line because it was longer
		// than its buffer.

		if l.crc != nil {
			// Error out early if there are multiple checksums.
			return 0, ArmorCorrupt
		}

		var expectedBytes [3]byte
		var m int
		m, err = base64.StdEncoding.Decode(expectedBytes[0:], line[len(line)-4:])
		if err != nil {
			return 0, fmt.Errorf("error decoding CRC: %s", err.Error())
		} else if m != 3 {
			return 0, fmt.Errorf("error decoding CRC: wrong size CRC")
		}

		crc := uint32(expectedBytes[0])<<16 |
			uint32(expectedBytes[1])<<8 |
			uint32(expectedBytes[2])
		l.crc = &crc

		line = line[:len(line)-5]

		lineWithChecksum = true

		// If we've found a checksum but there is still data left, we don't
		// want to enter the "looking for armor end" loop, we still need to
		// return the leftover data to the reader.
		foldedChecksum = len(line) > 0

		// At this point, `line` contains leftover data or "" (if checksum
		// was on separate line.)
	}

	expectArmorEnd := false
	if l.crc != nil && !foldedChecksum {
		// "looking for armor end" loop

		// We have a checksum, and we are now reading what comes afterwards.
		// Skip all empty lines until we see something and we except it to be
		// ArmorEnd at this point.

		// This loop is not entered if there is more data *before* the CRC
		// suffix (if the CRC is not on separate line).
		for {
			if len(strings.TrimSpace(string(line))) > 0 {
				break
			}
			lineWithChecksum = false
			line, _, err = l.in.ReadLine()
			if err == io.EOF {
				break
			}
			if err != nil {
				return
			}
		}
		expectArmorEnd = true
	}

	if bytes.HasPrefix(line, armorEnd) {
		if lineWithChecksum {
			// ArmorEnd and checksum at the same line?
			return 0, ArmorCorrupt
		}
		l.eof = true
		return 0, io.EOF
	} else if expectArmorEnd {
		// We wanted armorEnd but didn't see one.
		return 0, ArmorCorrupt
	}

	// Clean-up line from whitespace to pass it further (to base64
	// decoder). This is done after test for CRC and test for
	// armorEnd. Keys that have whitespace in CRC will have CRC
	// treated as part of the payload and probably fail in base64
	// reading.
	line = bytes.Map(func(r rune) rune {
		if ourIsSpace(r) {
			return -1
		}
		return r
	}, line)

	n = copy(p, line)
	bytesToSave := len(line) - n
	if bytesToSave > 0 {
		if cap(l.buf) < bytesToSave {
			l.buf = make([]byte, 0, bytesToSave)
		}
		l.buf = l.buf[0:bytesToSave]
		copy(l.buf, line[n:])
	}

	return
}

// openpgpReader passes Read calls to the underlying base64 decoder, but keeps
// a running CRC of the resulting data and checks the CRC against the value
// found by the lineReader at EOF.
type openpgpReader struct {
	lReader    *lineReader
	b64Reader  io.Reader
	currentCRC uint32
}

func (r *openpgpReader) Read(p []byte) (n int, err error) {
	n, err = r.b64Reader.Read(p)
	r.currentCRC = crc24(r.currentCRC, p[:n])

	if err == io.EOF {
		if r.lReader.crc != nil && *r.lReader.crc != uint32(r.currentCRC&crc24Mask) {
			return 0, ArmorCorrupt
		}
	}

	return
}

// Decode reads a PGP armored block from the given Reader. It will ignore
// leading garbage. If it doesn't find a block, it will return nil, io.EOF. The
// given Reader is not usable after calling this function: an arbitrary amount
// of data may have been read past the end of the block.
func Decode(in io.Reader) (p *Block, err error) {
	r := bufio.NewReaderSize(in, 100)
	var line []byte
	ignoreNext := false

TryNextBlock:
	p = nil

	// Skip leading garbage
	for {
		ignoreThis := ignoreNext
		line, ignoreNext, err = r.ReadLine()
		if err != nil {
			return
		}
		if ignoreNext || ignoreThis {
			continue
		}
		line = bytes.TrimSpace(line)
		if len(line) > len(armorStart)+len(armorEndOfLine) && bytes.HasPrefix(line, armorStart) {
			break
		}
	}

	p = new(Block)
	p.Type = string(line[len(armorStart) : len(line)-len(armorEndOfLine)])
	p.Header = make(map[string]string)
	nextIsContinuation := false
	var lastKey string

	// Read headers
	for {
		isContinuation := nextIsContinuation
		line, nextIsContinuation, err = r.ReadLine()
		if err != nil {
			p = nil
			return
		}
		if isContinuation {
			p.Header[lastKey] += string(line)
			continue
		}
		line = bytes.TrimFunc(line, ourIsSpace)
		if len(line) == 0 {
			break
		}

		i := bytes.Index(line, []byte(": "))
		if i == -1 {
			goto TryNextBlock
		}
		lastKey = string(line[:i])
		p.Header[lastKey] = string(line[i+2:])
	}

	p.lReader.in = r
	p.oReader.currentCRC = crc24Init
	p.oReader.lReader = &p.lReader
	p.oReader.b64Reader = base64.NewDecoder(base64.StdEncoding, &p.lReader)
	p.Body = &p.oReader

	return
}
