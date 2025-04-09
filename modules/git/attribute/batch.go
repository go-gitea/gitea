// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attribute

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// BatchChecker provides a reader for check-attribute content that can be long running
type BatchChecker struct {
	// params
	Attributes []string
	Repo       *git.Repository
	Treeish    string

	stdinReader io.ReadCloser
	stdinWriter *os.File
	stdOut      *nulSeparatedAttributeWriter
	cmd         *git.Command
	env         []string
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewBatchChecker creates a check attribute reader for the current repository and provided commit ID
func NewBatchChecker(repo *git.Repository, treeish string, attributes ...string) (*BatchChecker, error) {
	ctx, cancel := context.WithCancel(repo.Ctx)
	if len(attributes) == 0 {
		attributes = LinguistAttributes
	}
	checker := &BatchChecker{
		Attributes: attributes,
		Repo:       repo,
		Treeish:    treeish,
		ctx:        ctx,
		cancel:     cancel,
	}

	if err := checker.init(); err != nil {
		log.Error("Unable to open attribute checker for commit %s, error: %v", treeish, err)
		checker.Close()
		return nil, err
	}

	go func() {
		err := checker.run(ctx)
		if err != nil && !git.IsErrCanceledOrKilled(err) {
			log.Error("Attribute checker for commit %s exits with error: %v", treeish, err)
		}
		cancel()
	}()

	return checker, nil
}

// init initializes the AttributeChecker
func (c *BatchChecker) init() error {
	if len(c.Attributes) == 0 {
		lw := new(nulSeparatedAttributeWriter)
		lw.attributes = make(chan attributeTriple)
		lw.closed = make(chan struct{})

		c.stdOut = lw
		c.stdOut.Close()
		return errors.New("no provided Attributes to check")
	}

	cmd, envs, cancel, err := checkAttrCommand(c.Repo, c.Treeish, nil, c.Attributes)
	if err != nil {
		c.cancel()
		return err
	}
	c.cmd = cmd
	c.env = envs
	c.cancel = func() {
		cancel()
		c.cancel()
	}

	c.cmd.AddArguments("--stdin")

	c.stdinReader, c.stdinWriter, err = os.Pipe()
	if err != nil {
		c.cancel()
		return err
	}

	lw := new(nulSeparatedAttributeWriter)
	lw.attributes = make(chan attributeTriple, 5)
	lw.closed = make(chan struct{})
	c.stdOut = lw
	return nil
}

func (c *BatchChecker) run(ctx context.Context) error {
	defer func() {
		_ = c.stdinReader.Close()
		_ = c.stdOut.Close()
	}()
	stdErr := new(bytes.Buffer)
	err := c.cmd.Run(ctx, &git.RunOpts{
		Env:    c.env,
		Dir:    c.Repo.Path,
		Stdin:  c.stdinReader,
		Stdout: c.stdOut,
		Stderr: stdErr,
	})
	if err != nil && !git.IsErrCanceledOrKilled(err) {
		return fmt.Errorf("failed to run attr-check. Error: %w\nStderr: %s", err, stdErr.String())
	}
	return nil
}

// CheckPath check attr for given path
func (c *BatchChecker) CheckPath(path string) (rs Attributes, err error) {
	defer func() {
		if err != nil && err != c.ctx.Err() {
			log.Error("Unexpected error when checking path %s in %s, error: %v", path, filepath.Base(c.Repo.Path), err)
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
		debugMsg := fmt.Sprintf("check path %q in repo %q", path, filepath.Base(c.Repo.Path))
		debugMsg += fmt.Sprintf(", stdOut: tmp=%q, pos=%d, closed=%v", string(c.stdOut.tmp), c.stdOut.pos, stdOutClosed)
		// FIXME:
		//if c.cmd.cmd != nil {
		//	debugMsg += fmt.Sprintf(", process state: %q", c.cmd.cmd.ProcessState.String())
		//}
		_ = c.Close()
		return fmt.Errorf("CheckPath timeout: %s", debugMsg)
	}

	rs = make(map[string]Attribute)
	for range c.Attributes {
		select {
		case <-time.After(5 * time.Second):
			// There is a strange "hang" problem in gitdiff.GetDiff -> CheckPath
			// So add a timeout here to mitigate the problem, and output more logs for debug purpose
			// In real world, if CheckPath runs long than seconds, it blocks the end user's operation,
			// and at the moment the CheckPath result is not so important, so we can just ignore it.
			return nil, reportTimeout()
		case attr, ok := <-c.stdOut.ReadAttribute():
			if !ok {
				return nil, c.ctx.Err()
			}
			rs[attr.Attribute] = Attribute(attr.Value)
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
