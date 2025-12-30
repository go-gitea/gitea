// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package catfile

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// ErrObjectNotFound indicates that the requested object could not be read from cat-file
type ErrObjectNotFound struct {
	ID string
}

func (err ErrObjectNotFound) Error() string {
	return fmt.Sprintf("catfile: object does not exist [id: %s]", err.ID)
}

// IsErrObjectNotFound reports whether err is an ErrObjectNotFound
func IsErrObjectNotFound(err error) bool {
	var target ErrObjectNotFound
	return errors.As(err, &target)
}

// ObjectFormat abstracts the minimal information needed from git.ObjectFormat.
type ObjectFormat interface {
	FullLength() int
}

// ReadBatchLine reads the header line from cat-file --batch. It expects the format
// "<oid> SP <type> SP <size> LF" and leaves the reader positioned at the start of
// the object contents (which must be fully consumed by the caller).
func ReadBatchLine(rd *bufio.Reader) (sha []byte, typ string, size int64, err error) {
	typ, err = rd.ReadString('\n')
	if err != nil {
		return sha, typ, size, err
	}
	if len(typ) == 1 {
		typ, err = rd.ReadString('\n')
		if err != nil {
			return sha, typ, size, err
		}
	}
	idx := strings.IndexByte(typ, ' ')
	if idx < 0 {
		return sha, typ, size, ErrObjectNotFound{}
	}
	sha = []byte(typ[:idx])
	typ = typ[idx+1:]

	idx = strings.IndexByte(typ, ' ')
	if idx < 0 {
		return sha, typ, size, ErrObjectNotFound{ID: string(sha)}
	}

	sizeStr := typ[idx+1 : len(typ)-1]
	typ = typ[:idx]

	size, err = strconv.ParseInt(sizeStr, 10, 64)
	return sha, typ, size, err
}

// ReadTagObjectID reads a tag object ID hash from a cat-file --batch stream, throwing away the rest.
func ReadTagObjectID(rd *bufio.Reader, size int64) (string, error) {
	var id string
	var n int64
headerLoop:
	for {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			return "", err
		}
		n += int64(len(line))
		idx := bytes.Index(line, []byte{' '})
		if idx < 0 {
			continue
		}

		if string(line[:idx]) == "object" {
			id = string(line[idx+1 : len(line)-1])
			break headerLoop
		}
	}

	return id, DiscardFull(rd, size-n+1)
}

// ReadTreeID reads a tree ID from a cat-file --batch stream, throwing away the rest of the commit content.
func ReadTreeID(rd *bufio.Reader, size int64) (string, error) {
	var id string
	var n int64
headerLoop:
	for {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			return "", err
		}
		n += int64(len(line))
		idx := bytes.Index(line, []byte{' '})
		if idx < 0 {
			continue
		}

		if string(line[:idx]) == "tree" {
			id = string(line[idx+1 : len(line)-1])
			break headerLoop
		}
	}

	return id, DiscardFull(rd, size-n+1)
}

// hextable helps quickly convert between binary and hex representation
const hextable = "0123456789abcdef"

// BinToHex converts a binary hash into a hex encoded one. Input and output can be the
// same byte slice to support in-place conversion without allocations.
func BinToHex(objectFormat ObjectFormat, sha, out []byte) []byte {
	for i := objectFormat.FullLength()/2 - 1; i >= 0; i-- {
		v := sha[i]
		vhi, vlo := v>>4, v&0x0f
		shi, slo := hextable[vhi], hextable[vlo]
		out[i*2], out[i*2+1] = shi, slo
	}
	return out
}

// ParseCatFileTreeLine reads an entry from a tree in a cat-file --batch stream and avoids allocations
// where possible. Each line is composed of:
// <mode-in-ascii> SP <fname> NUL <binary HASH>
func ParseCatFileTreeLine(objectFormat ObjectFormat, rd *bufio.Reader, modeBuf, fnameBuf, shaBuf []byte) (mode, fname, sha []byte, n int, err error) {
	var readBytes []byte

	readBytes, err = rd.ReadSlice('\x00')
	if err != nil {
		return mode, fname, sha, n, err
	}
	idx := bytes.IndexByte(readBytes, ' ')
	if idx < 0 {
		log.Debug("missing space in readBytes ParseCatFileTreeLine: %s", readBytes)
		return mode, fname, sha, n, ErrObjectNotFound{}
	}

	n += idx + 1
	copy(modeBuf, readBytes[:idx])
	if len(modeBuf) >= idx {
		modeBuf = modeBuf[:idx]
	} else {
		modeBuf = append(modeBuf, readBytes[len(modeBuf):idx]...)
	}
	mode = modeBuf

	readBytes = readBytes[idx+1:]
	copy(fnameBuf, readBytes)
	if len(fnameBuf) > len(readBytes) {
		fnameBuf = fnameBuf[:len(readBytes)]
	} else {
		fnameBuf = append(fnameBuf, readBytes[len(fnameBuf):]...)
	}
	for err == bufio.ErrBufferFull {
		readBytes, err = rd.ReadSlice('\x00')
		fnameBuf = append(fnameBuf, readBytes...)
	}
	n += len(fnameBuf)
	if err != nil {
		return mode, fname, sha, n, err
	}
	fnameBuf = fnameBuf[:len(fnameBuf)-1]
	fname = fnameBuf

	idx = 0
	length := objectFormat.FullLength() / 2
	for idx < length {
		var read int
		read, err = rd.Read(shaBuf[idx:length])
		n += read
		if err != nil {
			return mode, fname, sha, n, err
		}
		idx += read
	}
	sha = shaBuf
	return mode, fname, sha, n, err
}

// DiscardFull discards the requested amount of bytes from the buffered reader regardless of its internal limit.
func DiscardFull(rd *bufio.Reader, discard int64) error {
	if discard > math.MaxInt32 {
		n, err := rd.Discard(math.MaxInt32)
		discard -= int64(n)
		if err != nil {
			return err
		}
	}
	for discard > 0 {
		n, err := rd.Discard(int(discard))
		discard -= int64(n)
		if err != nil {
			return err
		}
	}
	return nil
}
