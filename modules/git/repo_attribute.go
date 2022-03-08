// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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
	err := LoadGitVersion()
	if err != nil {
		return nil, fmt.Errorf("git version missing: %v", err)
	}

	env := []string{}

	if len(opts.IndexFile) > 0 && CheckGitVersionAtLeast("1.7.8") == nil {
		env = append(env, "GIT_INDEX_FILE="+opts.IndexFile)
	}
	if len(opts.WorkTree) > 0 && CheckGitVersionAtLeast("1.7.8") == nil {
		env = append(env, "GIT_WORK_TREE="+opts.WorkTree)
	}

	if len(env) > 0 {
		env = append(os.Environ(), env...)
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	cmdArgs := []string{"check-attr", "-z"}

	if opts.AllAttributes {
		cmdArgs = append(cmdArgs, "-a")
	} else {
		for _, attribute := range opts.Attributes {
			if attribute != "" {
				cmdArgs = append(cmdArgs, attribute)
			}
		}
	}

	// git check-attr --cached first appears in git 1.7.8
	if opts.CachedOnly && CheckGitVersionAtLeast("1.7.8") == nil {
		cmdArgs = append(cmdArgs, "--cached")
	}

	cmdArgs = append(cmdArgs, "--")

	for _, arg := range opts.Filenames {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmd := NewCommandContext(repo.Ctx, cmdArgs...)

	if err := cmd.RunInDirTimeoutEnvPipeline(env, -1, repo.Path, stdOut, stdErr); err != nil {
		return nil, fmt.Errorf("failed to run check-attr: %v\n%s\n%s", err, stdOut.String(), stdErr.String())
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
	running     chan struct{}
}

// Init initializes the cmd
func (c *CheckAttributeReader) Init(ctx context.Context) error {
	c.running = make(chan struct{})
	cmdArgs := []string{"check-attr", "--stdin", "-z"}

	if len(c.IndexFile) > 0 && CheckGitVersionAtLeast("1.7.8") == nil {
		cmdArgs = append(cmdArgs, "--cached")
		c.env = append(c.env, "GIT_INDEX_FILE="+c.IndexFile)
	}

	if len(c.WorkTree) > 0 && CheckGitVersionAtLeast("1.7.8") == nil {
		c.env = append(c.env, "GIT_WORK_TREE="+c.WorkTree)
	}

	c.env = append(c.env, "GIT_FLUSH=1")

	if len(c.Attributes) == 0 {
		lw := new(nulSeparatedAttributeWriter)
		lw.attributes = make(chan attributeTriple)
		lw.closed = make(chan struct{})

		c.stdOut = lw
		c.stdOut.Close()
		return fmt.Errorf("no provided Attributes to check")
	}

	cmdArgs = append(cmdArgs, c.Attributes...)
	cmdArgs = append(cmdArgs, "--")

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.cmd = NewCommandContext(c.ctx, cmdArgs...)

	var err error

	c.stdinReader, c.stdinWriter, err = os.Pipe()
	if err != nil {
		c.cancel()
		return err
	}

	if CheckGitVersionAtLeast("1.8.5") == nil {
		lw := new(nulSeparatedAttributeWriter)
		lw.attributes = make(chan attributeTriple, 5)
		lw.closed = make(chan struct{})
		c.stdOut = lw
	} else {
		lw := new(lineSeparatedAttributeWriter)
		lw.attributes = make(chan attributeTriple, 5)
		lw.closed = make(chan struct{})
		c.stdOut = lw
	}
	return nil
}

// Run run cmd
func (c *CheckAttributeReader) Run() error {
	defer func() {
		_ = c.stdinReader.Close()
		_ = c.stdOut.Close()
	}()
	stdErr := new(bytes.Buffer)
	err := c.cmd.RunInDirTimeoutEnvFullPipelineFunc(c.env, -1, c.Repo.Path, c.stdOut, stdErr, c.stdinReader, func(_ context.Context, _ context.CancelFunc) error {
		select {
		case <-c.running:
		default:
			close(c.running)
		}
		return nil
	})
	if err != nil && //                      If there is an error we need to return but:
		c.ctx.Err() != err && //             1. Ignore the context error if the context is cancelled or exceeds the deadline (RunWithContext could return c.ctx.Err() which is Canceled or DeadlineExceeded)
		err.Error() != "signal: killed" { // 2. We should not pass up errors due to the program being killed
		return fmt.Errorf("failed to run attr-check. Error: %w\nStderr: %s", err, stdErr.String())
	}
	return nil
}

// CheckPath check attr for given path
func (c *CheckAttributeReader) CheckPath(path string) (rs map[string]string, err error) {
	defer func() {
		if err != nil {
			log.Error("CheckPath returns error: %v", err)
		}
	}()

	select {
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	case <-c.running:
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
	select {
	case <-c.running:
	default:
		close(c.running)
	}
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

type lineSeparatedAttributeWriter struct {
	tmp        []byte
	attributes chan attributeTriple
	closed     chan struct{}
}

func (wr *lineSeparatedAttributeWriter) Write(p []byte) (n int, err error) {
	l := len(p)

	nlIdx := bytes.IndexByte(p, '\n')
	for nlIdx >= 0 {
		wr.tmp = append(wr.tmp, p[:nlIdx]...)

		if len(wr.tmp) == 0 {
			// This should not happen
			if len(p) > nlIdx+1 {
				wr.tmp = wr.tmp[:0]
				p = p[nlIdx+1:]
				nlIdx = bytes.IndexByte(p, '\n')
				continue
			} else {
				return l, nil
			}
		}

		working := attributeTriple{}
		if wr.tmp[0] == '"' {
			sb := new(strings.Builder)
			remaining := string(wr.tmp[1:])
			for len(remaining) > 0 {
				rn, _, tail, err := strconv.UnquoteChar(remaining, '"')
				if err != nil {
					if len(remaining) > 2 && remaining[0] == '"' && remaining[1] == ':' && remaining[2] == ' ' {
						working.Filename = sb.String()
						wr.tmp = []byte(remaining[3:])
						break
					}
					return l, fmt.Errorf("unexpected tail %s", string(remaining))
				}
				_, _ = sb.WriteRune(rn)
				remaining = tail
			}
		} else {
			idx := bytes.IndexByte(wr.tmp, ':')
			if idx < 0 {
				return l, fmt.Errorf("unexpected input %s", string(wr.tmp))
			}
			working.Filename = string(wr.tmp[:idx])
			if len(wr.tmp) < idx+2 {
				return l, fmt.Errorf("unexpected input %s", string(wr.tmp))
			}
			wr.tmp = wr.tmp[idx+2:]
		}

		idx := bytes.IndexByte(wr.tmp, ':')
		if idx < 0 {
			return l, fmt.Errorf("unexpected input %s", string(wr.tmp))
		}

		working.Attribute = string(wr.tmp[:idx])
		if len(wr.tmp) < idx+2 {
			return l, fmt.Errorf("unexpected input %s", string(wr.tmp))
		}

		working.Value = string(wr.tmp[idx+2:])

		wr.attributes <- working
		wr.tmp = wr.tmp[:0]
		if len(p) > nlIdx+1 {
			p = p[nlIdx+1:]
			nlIdx = bytes.IndexByte(p, '\n')
			continue
		} else {
			return l, nil
		}
	}

	wr.tmp = append(wr.tmp, p...)
	return l, nil
}

func (wr *lineSeparatedAttributeWriter) ReadAttribute() <-chan attributeTriple {
	return wr.attributes
}

func (wr *lineSeparatedAttributeWriter) Close() error {
	select {
	case <-wr.closed:
		return nil
	default:
	}
	close(wr.attributes)
	close(wr.closed)
	return nil
}
