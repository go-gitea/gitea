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
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

type catFileBatchCommunicator struct {
	closeFunc   atomic.Pointer[func(err error)]
	reqWriter   io.Writer
	respReader  *bufio.Reader
	debugGitCmd *gitcmd.Command
}

func (b *catFileBatchCommunicator) Close(err ...error) {
	if fn := b.closeFunc.Swap(nil); fn != nil {
		(*fn)(util.OptionalArg(err))
	}
}

// newCatFileBatch opens git cat-file --batch/--batch-check/--batch-command command and prepares the stdin/stdout pipes for communication.
func newCatFileBatch(ctx context.Context, repoPath string, cmdCatFile *gitcmd.Command) *catFileBatchCommunicator {
	ctx, ctxCancel := context.WithCancelCause(ctx)
	stdinWriter, stdoutReader, stdPipeClose := cmdCatFile.MakeStdinStdoutPipe()
	ret := &catFileBatchCommunicator{
		debugGitCmd: cmdCatFile,
		reqWriter:   stdinWriter,
		respReader:  bufio.NewReaderSize(stdoutReader, 32*1024), // use a buffered reader for rich operations
	}
	ret.closeFunc.Store(new(func(err error) {
		ctxCancel(err)
		stdPipeClose()
	}))

	err := cmdCatFile.WithDir(repoPath).StartWithStderr(ctx)
	if err != nil {
		log.Error("Unable to start git command %v: %v", cmdCatFile.LogString(), err)
		// ideally here it should return the error, but it would require refactoring all callers
		// so just return a dummy communicator that does nothing, almost the same behavior as before, not bad
		ret.Close(err)
		return ret
	}

	go func() {
		err := cmdCatFile.WaitWithStderr()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("cat-file --batch command failed in repo %s, error: %v", repoPath, err)
		}
		ret.Close(err)
	}()

	return ret
}

func (b *catFileBatchCommunicator) debugKill() (ret struct {
	beforeClose chan struct{}
	blockClose  chan struct{}
	afterClose  chan struct{}
},
) {
	ret.beforeClose = make(chan struct{})
	ret.blockClose = make(chan struct{})
	ret.afterClose = make(chan struct{})
	oldCloseFunc := b.closeFunc.Load()
	b.closeFunc.Store(new(func(err error) {
		b.closeFunc.Store(nil)
		close(ret.beforeClose)
		<-ret.blockClose
		(*oldCloseFunc)(err)
		close(ret.afterClose)
	}))
	b.debugGitCmd.DebugKill()
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

// ParseCatFileTreeLine reads an entry from a tree in a cat-file --batch stream
// Each entry is composed of:
// <mode-in-ascii-dropping-initial-zeros> SP <name> NUL <binary-hash>
func ParseCatFileTreeLine(objectFormat ObjectFormat, rd BufferedReader) (mode EntryMode, name string, objID ObjectID, n int, err error) {
	// use the in-buffer memory as much as possible to avoid extra allocations
	bufBytes, err := rd.ReadSlice('\x00')
	const maxEntryInfoBytes = 1024 * 1024
	if errors.Is(err, bufio.ErrBufferFull) {
		bufBytes = slices.Clone(bufBytes)
		for len(bufBytes) < maxEntryInfoBytes && errors.Is(err, bufio.ErrBufferFull) {
			var tmp []byte
			tmp, err = rd.ReadSlice('\x00')
			bufBytes = append(bufBytes, tmp...)
		}
	}
	if err != nil {
		return mode, name, objID, len(bufBytes), err
	}

	idx := bytes.IndexByte(bufBytes, ' ')
	if idx < 0 {
		return mode, name, objID, len(bufBytes), errors.New("invalid CatFileTreeLine output")
	}

	mode = ParseEntryMode(util.UnsafeBytesToString(bufBytes[:idx]))
	name = string(bufBytes[idx+1 : len(bufBytes)-1]) // trim the NUL terminator, it needs a copy because the bufBytes will be reused by the reader
	if mode == EntryModeNoEntry {
		return mode, name, objID, len(bufBytes), errors.New("invalid entry mode: " + string(bufBytes[:idx]))
	}

	switch objectFormat {
	case Sha1ObjectFormat:
		objID = &Sha1Hash{}
	case Sha256ObjectFormat:
		objID = &Sha256Hash{}
	default:
		panic("unsupported object format: " + objectFormat.Name())
	}
	readIDLen, err := io.ReadFull(rd, objID.RawValue())
	return mode, name, objID, len(bufBytes) + readIDLen, err
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
