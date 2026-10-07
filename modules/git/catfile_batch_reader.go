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

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

type catFileBatchCommunicator struct {
	closeFunc   atomic.Pointer[func(err error)]
	reqWriter   io.Writer
	respReader  *bufio.Reader
	content     *catFileContentReader
	debugGitCmd *gitcmd.Command
	closed      chan struct{}
}

type catFileContentReader struct {
	io.LimitedReader // R is nil once a newer query made the reader stale
}

func (r *catFileContentReader) Read(buf []byte) (int, error) {
	if r.R == nil {
		setting.PanicInDevOrTesting("cat-file content reader is used after a newer query on its batch")
		return 0, io.ErrClosedPipe
	}
	read, err := r.LimitedReader.Read(buf)
	if errors.Is(err, io.EOF) && r.N > 0 {
		err = io.ErrUnexpectedEOF
	}
	return read, err
}

// query discards the unread content of the previous query, then sends the request and reads the response header
func (b *catFileBatchCommunicator) query(request string) (*CatFileObject, error) {
	if b.content != nil {
		remaining := b.content.N + 1
		b.content.R = nil
		b.content = nil
		for remaining > 0 {
			discarded, err := b.respReader.Discard(int(min(remaining, math.MaxInt32)))
			remaining -= int64(discarded)
			if err != nil {
				return nil, err
			}
		}
	}
	if _, err := io.WriteString(b.reqWriter, request); err != nil {
		return nil, err
	}
	return catFileBatchParseInfoLine(b.respReader)
}

func (b *catFileBatchCommunicator) queryContent(request string) (*CatFileObject, io.Reader, error) {
	info, err := b.query(request)
	if err != nil {
		return nil, nil, err
	}
	b.content = &catFileContentReader{io.LimitedReader{R: b.respReader, N: info.Size}}
	return info, b.content, nil
}

func (b *catFileBatchCommunicator) Close(err ...error) {
	if fn := b.closeFunc.Swap(nil); fn != nil {
		(*fn)(util.OptionalArg(err))
	}
	// make sure the git process has fully exited before we return from Close()
	// otherwise, the opened files will block the directory renaming (rename a repo) on Windows
	<-b.closed
}

// newCatFileBatch opens git cat-file --batch/--batch-check/--batch-command command and prepares the stdin/stdout pipes for communication.
func newCatFileBatch(ctx context.Context, repo RepositoryFacade, cmdCatFile *gitcmd.Command) *catFileBatchCommunicator {
	ctx, ctxCancel := context.WithCancelCause(ctx)
	stdinWriter, stdoutReader, stdPipeClose := cmdCatFile.MakeStdinStdoutPipe()
	ret := &catFileBatchCommunicator{
		debugGitCmd: cmdCatFile,
		reqWriter:   stdinWriter,
		respReader:  bufio.NewReaderSize(stdoutReader, 32*1024), // use a buffered reader for rich operations
		closed:      make(chan struct{}),
	}
	ret.closeFunc.Store(new(func(err error) {
		ctxCancel(err)
		stdPipeClose()
	}))

	err := cmdCatFile.WithParentCallerInfo().WithRepo(repo).StartWithStderr(ctx)
	if err != nil {
		log.Error("Unable to start git command %v: %v", cmdCatFile.LogString(), err)
		// ideally here it should return the error, but it would require refactoring all callers
		// so just return a dummy communicator that does nothing, almost the same behavior as before, not bad
		close(ret.closed)
		ret.Close(err)
		return ret
	}

	go func() {
		err := cmdCatFile.WaitWithStderr()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("cat-file --batch command failed in repo %s, error: %v", repo.LogString(), err)
		}
		close(ret.closed)
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
func catFileBatchParseInfoLine(rd *bufio.Reader) (*CatFileObject, error) {
	typ, err := rd.ReadString('\n')
	if err != nil {
		return nil, err
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

func ReadTagObjectID(rd io.Reader) (string, error) {
	return readObjectHeader(rd, "object")
}

func ReadTreeID(rd io.Reader) (string, error) {
	return readObjectHeader(rd, "tree")
}

func readObjectHeader(rd io.Reader, key string) (string, error) {
	bufRd := bufio.NewReader(rd)
	for {
		line, err := bufRd.ReadBytes('\n')
		if err != nil {
			return "", err
		}
		if name, value, ok := bytes.Cut(line, []byte{' '}); ok && string(name) == key {
			return string(value[:len(value)-1]), nil
		}
	}
}

// ParseCatFileTreeLine reads an entry from a tree in a cat-file --batch stream
// Each entry is composed of:
// <mode-in-ascii-dropping-initial-zeros> SP <name> NUL <binary-hash>
func ParseCatFileTreeLine(objectFormat ObjectFormat, rd *bufio.Reader) (mode EntryMode, name string, objID ObjectID, n int, err error) {
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
