// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attribute

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// BatchChecker provides a reader for check-attribute content that can be long running
type BatchChecker struct {
	attributesNum int
	repo          *git.Repository
	stdinWriter   *os.File
	stdOut        *nulSeparatedAttributeWriter
	ctx           context.Context
	cancel        context.CancelFunc
	cmd           *git.Command
}

// NewBatchChecker creates a check attribute reader for the current repository and provided commit ID
// If treeish is empty, then it will use current working directory, otherwise it will use the provided treeish on the bare repo
func NewBatchChecker(repo *git.Repository, treeish string, attributes []string) (checker *BatchChecker, returnedErr error) {
	ctx, cancel := context.WithCancel(repo.Ctx)
	defer func() {
		if returnedErr != nil {
			cancel()
		}
	}()

	cmd, envs, cleanup, err := checkAttrCommand(repo, treeish, nil, attributes)
	if err != nil {
		return nil, err
	}
	defer func() {
		if returnedErr != nil {
			cleanup()
		}
	}()

	cmd.AddArguments("--stdin")

	checker = &BatchChecker{
		attributesNum: len(attributes),
		repo:          repo,
		ctx:           ctx,
		cmd:           cmd,
		cancel: func() {
			cancel()
			cleanup()
		},
	}

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	checker.stdinWriter = stdinWriter

	lw := new(nulSeparatedAttributeWriter)
	lw.attributes = make(chan attributeTriple, len(attributes))
	lw.closed = make(chan struct{})
	checker.stdOut = lw

	go func() {
		defer func() {
			_ = stdinReader.Close()
			_ = lw.Close()
		}()
		stdErr := new(bytes.Buffer)
		err := cmd.Run(ctx, &git.RunOpts{
			Env:    envs,
			Dir:    repo.Path,
			Stdin:  stdinReader,
			Stdout: lw,
			Stderr: stdErr,
		})

		if err != nil && !git.IsErrCanceledOrKilled(err) {
			log.Error("Attribute checker for commit %s exits with error: %v", treeish, err)
		}
		checker.cancel()
	}()

	return checker, nil
}

// CheckPath check attr for given path
func (c *BatchChecker) CheckPath(path string) (rs *Attributes, err error) {
	defer func() {
		if err != nil && err != c.ctx.Err() {
			log.Error("Unexpected error when checking path %s in %s, error: %v", path, filepath.Base(c.repo.Path), err)
		}
	}()

	select {
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	default:
	}

	if _, err = c.stdinWriter.Write([]byte(path + "\x00")); err != nil {
		defer c.Close()
		return nil, err
	}

	reportTimeout := func() error {
		stdOutClosed := false
		select {
		case <-c.stdOut.closed:
			stdOutClosed = true
		default:
		}
		debugMsg := fmt.Sprintf("check path %q in repo %q", path, filepath.Base(c.repo.Path))
		debugMsg += fmt.Sprintf(", stdOut: tmp=%q, pos=%d, closed=%v", string(c.stdOut.tmp), c.stdOut.pos, stdOutClosed)
		if c.cmd != nil {
			debugMsg += fmt.Sprintf(", process state: %q", c.cmd.ProcessState())
		}
		_ = c.Close()
		return fmt.Errorf("CheckPath timeout: %s", debugMsg)
	}

	rs = NewAttributes()
	for i := 0; i < c.attributesNum; i++ {
		select {
		case <-time.After(5 * time.Second):
			// there is no "hang" problem now. This code is just used to catch other potential problems.
			return nil, reportTimeout()
		case attr, ok := <-c.stdOut.ReadAttribute():
			if !ok {
				return nil, c.ctx.Err()
			}
			rs.m[attr.Attribute] = Attribute(attr.Value)
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		}
	}
	return rs, nil
}

func (c *BatchChecker) Close() error {
	c.cancel()
	err := c.stdinWriter.Close()
	return err
}

type attributeTriple struct {
	Filename  string
	Attribute string
	Value     string
}

type nulSeparatedAttributeWriter struct {
	tmp        []byte
	attributes chan attributeTriple
	closed     chan struct{}
	working    attributeTriple
	pos        int
}

func (wr *nulSeparatedAttributeWriter) Write(p []byte) (n int, err error) {
	l, read := len(p), 0

	nulIdx := bytes.IndexByte(p, '\x00')
	for nulIdx >= 0 {
		wr.tmp = append(wr.tmp, p[:nulIdx]...)
		switch wr.pos {
		case 0:
			wr.working = attributeTriple{
				Filename: string(wr.tmp),
			}
		case 1:
			wr.working.Attribute = string(wr.tmp)
		case 2:
			wr.working.Value = string(wr.tmp)
		}
		wr.tmp = wr.tmp[:0]
		wr.pos++
		if wr.pos > 2 {
			wr.attributes <- wr.working
			wr.pos = 0
		}
		read += nulIdx + 1
		if l > read {
			p = p[nulIdx+1:]
			nulIdx = bytes.IndexByte(p, '\x00')
		} else {
			return l, nil
		}
	}
	wr.tmp = append(wr.tmp, p...)
	return l, nil
}

func (wr *nulSeparatedAttributeWriter) ReadAttribute() <-chan attributeTriple {
	return wr.attributes
}

func (wr *nulSeparatedAttributeWriter) Close() error {
	select {
	case <-wr.closed:
		return nil
	default:
	}
	close(wr.attributes)
	close(wr.closed)
	return nil
}
