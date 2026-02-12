// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
)

var catFileBatchDebugWaitClose atomic.Int64

type catFileBatchCommunicator struct {
	cancel      context.CancelFunc
	reqWriter   io.Writer
	respReader  *bufio.Reader
	debugGitCmd *gitcmd.Command
}

func (b *catFileBatchCommunicator) Close() {
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
}

// newCatFileBatch opens git cat-file --batch in the provided repo and returns a stdin pipe, a stdout reader and cancel function
func newCatFileBatch(ctx context.Context, repoPath string, cmdCatFile *gitcmd.Command) (ret *catFileBatchCommunicator) {
	ctx, ctxCancel := context.WithCancelCause(ctx)

	// We often want to feed the commits in order into cat-file --batch, followed by their trees and subtrees as necessary.
	stdinWriter, stdoutReader, stdPipeClose := cmdCatFile.MakeStdinStdoutPipe()
	pipeClose := func() {
		if delay := catFileBatchDebugWaitClose.Load(); delay > 0 {
			time.Sleep(time.Duration(delay)) // for testing purpose only
		}
		stdPipeClose()
	}

	ret = &catFileBatchCommunicator{
		debugGitCmd: cmdCatFile,
		cancel:      func() { ctxCancel(nil) },
		reqWriter:   stdinWriter,
		respReader:  bufio.NewReaderSize(stdoutReader, 32*1024), // use a buffered reader for rich operations
	}

	err := cmdCatFile.WithDir(repoPath).StartWithStderr(ctx)
	if err != nil {
		log.Error("Unable to start git command %v: %v", cmdCatFile.LogString(), err)
		// ideally here it should return the error, but it would require refactoring all callers
		// so just return a dummy communicator that does nothing, almost the same behavior as before, not bad
		ctxCancel(err)
		pipeClose()
		return ret
	}

	go func() {
		err := cmdCatFile.WaitWithStderr()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("cat-file --batch command failed in repo %s, error: %v", repoPath, err)
		}
		ctxCancel(err)
		pipeClose()
	}()

	return ret
}

// catFileBatchParseInfoLine reads the header line from cat-file --batch
// We expect: <oid> SP <type> SP <size> LF
// then leaving the rest of the stream "<contents> LF" to be read
func catFileBatchParseInfoLine(rd BufferedReader) (*CatFileObject, error) {
	typ, err := rd.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(typ) == 1 {
		typ, err = rd.ReadString('\n')
		if err != nil {
			return nil, err
		}
	}
	idx := strings.IndexByte(typ, ' ')
	if idx < 0 {
		return nil, ErrNotExist{}
	}
	sha := typ[:idx]
	typ = typ[idx+1:]

	idx = strings.IndexByte(typ, ' ')
	if idx < 0 {
		return nil, ErrNotExist{ID: sha}
	}

	sizeStr := typ[idx+1 : len(typ)-1]
	typ = typ[:idx]

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	return &CatFileObject{ID: sha, Type: typ, Size: size}, err
}

// ReadTagObjectID reads a tag object ID hash from a cat-file --batch stream, throwing away the rest of the stream.
func ReadTagObjectID(rd BufferedReader, size int64) (string, error) {
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

	// Discard the rest of the tag
	return id, DiscardFull(rd, size-n+1)
}

// ReadTreeID reads a tree ID from a cat-file --batch stream, throwing away the rest of the stream.
func ReadTreeID(rd BufferedReader, size int64) (string, error) {
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

	// Discard the rest of the commit
	return id, DiscardFull(rd, size-n+1)
}

// git tree files are a list:
// <mode-in-ascii> SP <fname> NUL <binary Hash>
//
// Unfortunately this 20-byte notation is somewhat in conflict to all other git tools
// Therefore we need some method to convert these binary hashes to hex hashes

// ParseCatFileTreeLine reads an entry from a tree in a cat-file --batch stream
// This carefully avoids allocations - except where fnameBuf is too small.
// It is recommended therefore to pass in an fnameBuf large enough to avoid almost all allocations
//
// Each line is composed of:
// <mode-in-ascii-dropping-initial-zeros> SP <fname> NUL <binary HASH>
//
// We don't attempt to convert the raw HASH to save a lot of time
func ParseCatFileTreeLine(objectFormat ObjectFormat, rd BufferedReader, modeBuf, fnameBuf, shaBuf []byte) (mode, fname, sha []byte, n int, err error) {
	var readBytes []byte

	// Read the Mode & fname
	readBytes, err = rd.ReadSlice('\x00')
	if err != nil {
		return mode, fname, sha, n, err
	}
	idx := bytes.IndexByte(readBytes, ' ')
	if idx < 0 {
		log.Debug("missing space in readBytes ParseCatFileTreeLine: %s", readBytes)
		return mode, fname, sha, n, &ErrNotExist{}
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

	// Deal with the fname
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

	// Deal with the binary hash
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

func DiscardFull(rd BufferedReader, discard int64) error {
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
