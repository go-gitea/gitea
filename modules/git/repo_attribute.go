// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"code.gitea.io/gitea/modules/log"
)

// CheckAttributeOpts represents the possible options to CheckAttribute
type CheckAttributeOpts struct {
	CachedOnly    bool
	AllAttributes bool
	Attributes    []string
	Filenames     []string
	IndexFile     string
	WorkTree      string
}

// CheckAttribute return the Blame object of file
func (repo *Repository) CheckAttribute(opts CheckAttributeOpts) (map[string]map[string]string, error) {
	env := []string{}

	if len(opts.IndexFile) > 0 {
		env = append(env, "GIT_INDEX_FILE="+opts.IndexFile)
	}
	if len(opts.WorkTree) > 0 {
		env = append(env, "GIT_WORK_TREE="+opts.WorkTree)
	}

	if len(env) > 0 {
		env = append(os.Environ(), env...)
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	cmd := NewCommand("check-attr", "-z")

	if opts.AllAttributes {
		cmd.AddArguments("-a")
	} else {
		for _, attribute := range opts.Attributes {
			if attribute != "" {
				cmd.AddDynamicArguments(attribute)
			}
		}
	}

	if opts.CachedOnly {
		cmd.AddArguments("--cached")
	}

	cmd.AddDashesAndList(opts.Filenames...)

	if err := cmd.Run(repo.Ctx, &RunOpts{
		Env:    env,
		Dir:    repo.Path,
		Stdout: stdOut,
		Stderr: stdErr,
	}); err != nil {
		return nil, fmt.Errorf("failed to run check-attr: %w\n%s\n%s", err, stdOut.String(), stdErr.String())
	}

	// FIXME: This is incorrect on versions < 1.8.5
	fields := bytes.Split(stdOut.Bytes(), []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, fmt.Errorf("wrong number of fields in return from check-attr")
	}

	name2attribute2info := make(map[string]map[string]string)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := name2attribute2info[filename]
		if attribute2info == nil {
			attribute2info = make(map[string]string)
		}
		attribute2info[attribute] = info
		name2attribute2info[filename] = attribute2info
	}

	return name2attribute2info, nil
}

// CheckAttributeReader provides a reader for check-attribute content that can be long running
type CheckAttributeReader struct {
	// params
	Attributes []string
	Repo       *Repository
	IndexFile  string
	WorkTree   string

	stdinReader io.ReadCloser
	stdinWriter *os.File
	stdOut      attributeWriter
	cmd         *Command
	env         []string
	ctx         context.Context
	cancel      context.CancelFunc
}

// Init initializes the CheckAttributeReader
func (c *CheckAttributeReader) Init(ctx context.Context) error {
	if len(c.Attributes) == 0 {
		lw := new(nulSeparatedAttributeWriter)
		lw.attributes = make(chan attributeTriple)
		lw.closed = make(chan struct{})

		c.stdOut = lw
		c.stdOut.Close()
		return fmt.Errorf("no provided Attributes to check")
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.cmd = NewCommand("check-attr", "--stdin", "-z")

	if len(c.IndexFile) > 0 {
		c.cmd.AddArguments("--cached")
		c.env = append(c.env, "GIT_INDEX_FILE="+c.IndexFile)
	}

	if len(c.WorkTree) > 0 {
		c.env = append(c.env, "GIT_WORK_TREE="+c.WorkTree)
	}

	c.env = append(c.env, "GIT_FLUSH=1")

	c.cmd.AddDynamicArguments(c.Attributes...)

	var err error

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

// Run run cmd
func (c *CheckAttributeReader) Run() error {
	defer func() {
		_ = c.stdinReader.Close()
		_ = c.stdOut.Close()
	}()
	stdErr := new(bytes.Buffer)
	err := c.cmd.Run(c.ctx, &RunOpts{
		Env:    c.env,
		Dir:    c.Repo.Path,
		Stdin:  c.stdinReader,
		Stdout: c.stdOut,
		Stderr: stdErr,
	})
	if err != nil && !IsErrCanceledOrKilled(err) {
		return fmt.Errorf("failed to run attr-check. Error: %w\nStderr: %s", err, stdErr.String())
	}
	return nil
}

// CheckPath check attr for given path
func (c *CheckAttributeReader) CheckPath(path string) (rs map[string]string, err error) {
	defer func() {
		if err != nil && err != c.ctx.Err() {
			log.Error("Unexpected error when checking path %s in %s. Error: %v", path, c.Repo.Path, err)
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

	rs = make(map[string]string)
	for range c.Attributes {
		select {
		case attr, ok := <-c.stdOut.ReadAttribute():
			if !ok {
				return nil, c.ctx.Err()
			}
			rs[attr.Attribute] = attr.Value
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		}
	}
	return rs, nil
}

// Close close pip after use
func (c *CheckAttributeReader) Close() error {
	c.cancel()
	err := c.stdinWriter.Close()
	return err
}

type attributeWriter interface {
	io.WriteCloser
	ReadAttribute() <-chan attributeTriple
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
	return len(p), nil
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

// Create a check attribute reader for the current repository and provided commit ID
func (repo *Repository) CheckAttributeReader(commitID string) (*CheckAttributeReader, context.CancelFunc) {
	indexFilename, worktree, deleteTemporaryFile, err := repo.ReadTreeToTemporaryIndex(commitID)
	if err != nil {
		return nil, func() {}
	}

	checker := &CheckAttributeReader{
		Attributes: []string{
			AttributeLinguistVendored,
			AttributeLinguistGenerated,
			AttributeLinguistDocumentation,
			AttributeLinguistDetectable,
			AttributeLinguistLanguage,
			AttributeGitlabLanguage,
		},
		Repo:      repo,
		IndexFile: indexFilename,
		WorkTree:  worktree,
	}
	ctx, cancel := context.WithCancel(repo.Ctx)
	if err := checker.Init(ctx); err != nil {
		log.Error("Unable to open checker for %s. Error: %v", commitID, err)
	} else {
		go func() {
			err := checker.Run()
			if err != nil && err != ctx.Err() {
				log.Error("Unable to open checker for %s. Error: %v", commitID, err)
			}
			cancel()
		}()
	}
	deferable := func() {
		_ = checker.Close()
		cancel()
		deleteTemporaryFile()
	}

	return checker, deferable
}
