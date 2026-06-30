// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"path"
	"sort"
	"strings"

	"gitea.dev/modules/container"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
)

// logNameStatusRepo opens git log --raw in the provided repo and returns a parser
func logNameStatusRepo(ctx context.Context, repository, head, treepath string, paths ...string) *logNameStatusRepoParser {
	cmd := gitcmd.NewCommand()
	cmd.AddArguments("log", "--name-status", "-c", "--format=commit%x00%H %P%x00", "--parents", "--no-renames", "-t", "-z").AddDynamicArguments(head)

	var files []string
	if len(paths) < 70 {
		if treepath != "" {
			files = append(files, treepath)
			for _, pth := range paths {
				if pth != "" {
					files = append(files, path.Join(treepath, pth))
				}
			}
		} else {
			for _, pth := range paths {
				if pth != "" {
					files = append(files, pth)
				}
			}
		}
	} else if treepath != "" {
		files = append(files, treepath)
	}
	// Use the :(literal) pathspec magic to handle edge cases with files named like ":file.txt" or "*.jpg"
	for i, file := range files {
		files[i] = ":(literal)" + file
	}
	cmd.AddDashesAndList(files...)

	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	ctx, ctxCancel := context.WithCancel(ctx)
	go func() {
		err := cmd.WithDir(repository).RunWithStderr(ctx)
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			log.Error("Unable to run git command %v: %v", cmd.LogString(), err)
		}
	}()

	bufReader := bufio.NewReaderSize(stdoutReader, 32*1024)
	return &logNameStatusRepoParser{
		treepath: treepath,
		paths:    paths,
		rd:       bufReader,
		close: func() {
			ctxCancel()
			stdoutReaderClose()
		},
	}
}

// logNameStatusRepoParser parses a git log raw output from LogRawRepo
type logNameStatusRepoParser struct {
	treepath string
	paths    []string
	next     []byte
	buffull  bool
	rd       *bufio.Reader
	close    func()
}

// logNameStatusCommitData represents a commit artifact from git log raw
type logNameStatusCommitData struct {
	CommitID  string
	ParentIDs []string
	Paths     []bool
}

// walkNext returns the next LogStatusCommitData
func (g *logNameStatusRepoParser) walkNext(treepath string, paths2ids map[string]int, changed []bool, maxpathlen int) (*logNameStatusCommitData, error) {
	var err error
	if len(g.next) == 0 {
		g.buffull = false
		g.next, err = g.rd.ReadSlice('\x00')
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			g.buffull = true
		case err != nil:
			return nil, err
		}
	}

	ret := logNameStatusCommitData{}
	if bytes.Equal(g.next, []byte("commit\000")) {
		g.next, err = g.rd.ReadSlice('\x00')
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			g.buffull = true
		case err != nil:
			return nil, err
		}
	}

	// Our "line" must look like: <commitid> SP (<parent> SP) * NUL
	commitIDs := string(g.next)
	if g.buffull {
		more, err := g.rd.ReadString('\x00')
		if err != nil {
			return nil, err
		}
		commitIDs += more
	}
	commitIDs = commitIDs[:len(commitIDs)-1]
	splitIDs := strings.Split(commitIDs, " ")
	ret.CommitID = splitIDs[0]
	if len(splitIDs) > 1 {
		ret.ParentIDs = splitIDs[1:]
	}

	// now read the next "line"
	g.buffull = false
	g.next, err = g.rd.ReadSlice('\x00')
	if err != nil {
		if err == bufio.ErrBufferFull {
			g.buffull = true
		} else if err != io.EOF {
			return nil, err
		}
	}

	if err == io.EOF || !(g.next[0] == '\n' || g.next[0] == '\000') {
		return &ret, nil
	}

	// Ok we have some changes.
	// This line will look like: NL <fname> NUL
	//
	// Subsequent lines will not have the NL - so drop it here - g.bufffull must also be false at this point too.
	if g.next[0] == '\n' {
		g.next = g.next[1:]
	} else {
		g.buffull = false
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err != io.EOF {
				return nil, err
			}
		}
		if len(g.next) == 0 {
			return &ret, nil
		}
		if g.next[0] == '\x00' {
			g.buffull = false
			g.next, err = g.rd.ReadSlice('\x00')
			if err != nil {
				if err == bufio.ErrBufferFull {
					g.buffull = true
				} else if err != io.EOF {
					return nil, err
				}
			}
		}
	}

	fnameBuf := make([]byte, 4096)

diffloop:
	for {
		if err == io.EOF || bytes.Equal(g.next, []byte("commit\000")) {
			return &ret, nil
		}
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			switch err {
			case bufio.ErrBufferFull:
				g.buffull = true
			case io.EOF:
				return &ret, nil
			default:
				return nil, err
			}
		}
		copy(fnameBuf, g.next)
		if len(fnameBuf) < len(g.next) {
			fnameBuf = append(fnameBuf, g.next[len(fnameBuf):]...)
		} else {
			fnameBuf = fnameBuf[:len(g.next)]
		}
		if err != nil {
			if err != bufio.ErrBufferFull {
				return nil, err
			}
			more, err := g.rd.ReadBytes('\x00')
			if err != nil {
				return nil, err
			}
			fnameBuf = append(fnameBuf, more...)
		}

		// read the next line
		g.buffull = false
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err != io.EOF {
				return nil, err
			}
		}

		if treepath != "" {
			if !bytes.HasPrefix(fnameBuf, []byte(treepath)) {
				fnameBuf = fnameBuf[:cap(fnameBuf)]
				continue diffloop
			}
		}
		fnameBuf = fnameBuf[len(treepath) : len(fnameBuf)-1]
		if len(fnameBuf) > maxpathlen {
			fnameBuf = fnameBuf[:cap(fnameBuf)]
			continue diffloop
		}
		if len(fnameBuf) > 0 {
			if len(treepath) > 0 {
				if fnameBuf[0] != '/' || bytes.IndexByte(fnameBuf[1:], '/') >= 0 {
					fnameBuf = fnameBuf[:cap(fnameBuf)]
					continue diffloop
				}
				fnameBuf = fnameBuf[1:]
			} else if bytes.IndexByte(fnameBuf, '/') >= 0 {
				fnameBuf = fnameBuf[:cap(fnameBuf)]
				continue diffloop
			}
		}

		idx, ok := paths2ids[string(fnameBuf)]
		if !ok {
			fnameBuf = fnameBuf[:cap(fnameBuf)]
			continue diffloop
		}
		if ret.Paths == nil {
			ret.Paths = changed
		}
		changed[idx] = true
	}
}

var walkGitLogDebugBeforeNext func() // is used to simulate various edge git process cases

// walkGitLog walks the git log --name-status for the head commit in the provided treepath and files
func walkGitLog(ctx context.Context, repo *Repository, head *Commit, treepath string, paths ...string) (map[string]string, error) {
	headRef := head.ID.String()

	tree, err := head.SubTree(treepath)
	if err != nil {
		return nil, err
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		paths = make([]string, 0, len(entries)+1)
		paths = append(paths, "")
		for _, entry := range entries {
			paths = append(paths, entry.Name())
		}
	} else {
		sort.Strings(paths)
		if paths[0] != "" {
			paths = append([]string{""}, paths...)
		}
		// remove duplicates
		for i := len(paths) - 1; i > 0; i-- {
			if paths[i] == paths[i-1] {
				paths = append(paths[:i-1], paths[i:]...)
			}
		}
	}

	path2idx := map[string]int{}
	maxpathlen := len(treepath)

	for i := range paths {
		path2idx[paths[i]] = i
		pthlen := len(paths[i]) + len(treepath) + 1
		if pthlen > maxpathlen {
			maxpathlen = pthlen
		}
	}

	g := logNameStatusRepo(ctx, repo.Path, head.ID.String(), treepath, paths...)
	// don't use defer g.cancel() here as g may change its value - instead wrap in a func
	defer func() { g.close() }()

	results := make([]string, len(paths))
	remaining := len(paths)
	nextRestart := min((len(paths)*3)/4, 70)
	lastEmptyParent := head.ID.String()
	commitSinceLastEmptyParent := uint64(0)
	commitSinceNextRestart := uint64(0)
	parentRemaining := make(container.Set[string])

	changed := make([]bool, len(paths))

heaploop:
	for {
		if walkGitLogDebugBeforeNext != nil {
			walkGitLogDebugBeforeNext()
		}
		current, err := g.walkNext(treepath, path2idx, changed, maxpathlen)
		if ctx.Err() != nil {
			break heaploop // context is either canceled or deadline exceeded - break the loop and return what we have so far
		} else if errors.Is(err, io.EOF) {
			break heaploop // reached to the end of log output
		} else if err != nil {
			return nil, err // other unknown errors
		}
		parentRemaining.Remove(current.CommitID)
		for i, found := range current.Paths {
			if !found {
				continue
			}
			changed[i] = false
			if results[i] == "" {
				results[i] = current.CommitID
				if err := repo.LastCommitCache.Put(headRef, path.Join(treepath, paths[i]), current.CommitID); err != nil {
					return nil, err
				}
				delete(path2idx, paths[i])
				remaining--
				if results[0] == "" {
					results[0] = current.CommitID
					if err := repo.LastCommitCache.Put(headRef, treepath, current.CommitID); err != nil {
						return nil, err
					}
					delete(path2idx, "")
					remaining--
				}
			}
		}

		if remaining <= 0 {
			break heaploop
		}
		commitSinceLastEmptyParent++
		if len(parentRemaining) == 0 {
			lastEmptyParent = current.CommitID
			commitSinceLastEmptyParent = 0
		}
		if remaining <= nextRestart {
			commitSinceNextRestart++
			if 4*commitSinceNextRestart > 3*commitSinceLastEmptyParent {
				remainingPaths := make([]string, 0, len(paths))
				for i, pth := range paths {
					if results[i] == "" {
						remainingPaths = append(remainingPaths, pth)
					}
				}
				g.close()
				g = logNameStatusRepo(ctx, repo.Path, lastEmptyParent, treepath, remainingPaths...)
				parentRemaining = make(container.Set[string])
				nextRestart = (remaining * 3) / 4
				continue heaploop
			}
		}
		parentRemaining.AddMultiple(current.ParentIDs...)
	}

	resultsMap := map[string]string{}
	for i, pth := range paths {
		resultsMap[pth] = results[i]
	}

	return resultsMap, nil
}
