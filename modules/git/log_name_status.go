// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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

	"code.gitea.io/gitea/modules/container"

	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
)

// LogNameStatusRepo opens git log --raw in the provided repo and returns a stdin pipe, a stdout reader and cancel function
func LogNameStatusRepo(ctx context.Context, repository, head, treepath string, paths ...string) (*bufio.Reader, func()) {
	// We often want to feed the commits in order into cat-file --batch, followed by their trees and sub trees as necessary.
	// so let's create a batch stdin and stdout
	stdoutReader, stdoutWriter := nio.Pipe(buffer.New(32 * 1024))

	// Lets also create a context so that we can absolutely ensure that the command should die when we're done
	ctx, ctxCancel := context.WithCancel(ctx)

	cancel := func() {
		ctxCancel()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}

	cmd := NewCommand(ctx)
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

	go func() {
		stderr := strings.Builder{}
		err := cmd.Run(&RunOpts{
			Dir:    repository,
			Stdout: stdoutWriter,
			Stderr: &stderr,
		})
		if err != nil {
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
			return
		}

		_ = stdoutWriter.Close()
	}()

	// For simplicities sake we'll us a buffered reader to read from the cat-file --batch
	bufReader := bufio.NewReaderSize(stdoutReader, 32*1024)

	return bufReader, cancel
}

// LogNameStatusRepoParser parses a git log raw output from LogRawRepo
type LogNameStatusRepoParser struct {
	treepath string
	paths    []string
	next     []byte
	buffull  bool
	rd       *bufio.Reader
	cancel   func()
}

// NewLogNameStatusRepoParser returns a new parser for a git log raw output
func NewLogNameStatusRepoParser(ctx context.Context, repository, head, treepath string, paths ...string) *LogNameStatusRepoParser {
	rd, cancel := LogNameStatusRepo(ctx, repository, head, treepath, paths...)
	return &LogNameStatusRepoParser{
		treepath: treepath,
		paths:    paths,
		rd:       rd,
		cancel:   cancel,
	}
}

// LogNameStatusCommitData represents a commit artefact from git log raw
type LogNameStatusCommitData struct {
	CommitID  string
	ParentIDs []string
	Paths     []bool
}

// Next returns the next LogStatusCommitData
func (g *LogNameStatusRepoParser) Next(treepath string, paths2ids map[string]int, changed []bool, maxpathlen int) (*LogNameStatusCommitData, error) {
	var err error
	if g.next == nil || len(g.next) == 0 {
		g.buffull = false
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err == io.EOF {
				return nil, nil
			} else {
				return nil, err
			}
		}
	}

	ret := LogNameStatusCommitData{}
	if bytes.Equal(g.next, []byte("commit\000")) {
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err == io.EOF {
				return nil, nil
			} else {
				return nil, err
			}
		}
	}

	// Our "line" must look like: <commitid> SP (<parent> SP) * NUL
	ret.CommitID = string(g.next[0:40])
	parents := string(g.next[41:])
	if g.buffull {
		more, err := g.rd.ReadString('\x00')
		if err != nil {
			return nil, err
		}
		parents += more
	}
	parents = parents[:len(parents)-1]
	ret.ParentIDs = strings.Split(parents, " ")

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
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err == io.EOF {
				return &ret, nil
			} else {
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

// Close closes the parser
func (g *LogNameStatusRepoParser) Close() {
	g.cancel()
}

// WalkGitLog walks the git log --name-status for the head commit in the provided treepath and files
func WalkGitLog(ctx context.Context, repo *Repository, head *Commit, treepath string, paths ...string) (map[string]string, error) {
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

	g := NewLogNameStatusRepoParser(ctx, repo.Path, head.ID.String(), treepath, paths...)
	// don't use defer g.Close() here as g may change its value - instead wrap in a func
	defer func() {
		g.Close()
	}()

	results := make([]string, len(paths))
	remaining := len(paths)
	nextRestart := (len(paths) * 3) / 4
	if nextRestart > 70 {
		nextRestart = 70
	}
	lastEmptyParent := head.ID.String()
	commitSinceLastEmptyParent := uint64(0)
	commitSinceNextRestart := uint64(0)
	parentRemaining := make(container.Set[string])

	changed := make([]bool, len(paths))

heaploop:
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				break heaploop
			}
			g.Close()
			return nil, ctx.Err()
		default:
		}
		current, err := g.Next(treepath, path2idx, changed, maxpathlen)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				break heaploop
			}
			g.Close()
			return nil, err
		}
		if current == nil {
			break heaploop
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
				g.Close()
				remainingPaths := make([]string, 0, len(paths))
				for i, pth := range paths {
					if results[i] == "" {
						remainingPaths = append(remainingPaths, pth)
					}
				}
				g = NewLogNameStatusRepoParser(ctx, repo.Path, lastEmptyParent, treepath, remainingPaths...)
				parentRemaining = make(container.Set[string])
				nextRestart = (remaining * 3) / 4
				continue heaploop
			}
		}
		parentRemaining.AddMultiple(current.ParentIDs...)
	}
	g.Close()

	resultsMap := map[string]string{}
	for i, pth := range paths {
		resultsMap[pth] = results[i]
	}

	return resultsMap, nil
}
