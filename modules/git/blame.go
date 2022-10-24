// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"code.gitea.io/gitea/modules/process"
)

// BlamePart represents block of blame - continuous lines with one sha
type BlamePart struct {
	Sha   string
	Lines []string
}

// BlameReader returns part of file blame one by one
type BlameReader struct {
	cmd      *exec.Cmd
	output   io.ReadCloser
	reader   *bufio.Reader
	lastSha  *string
	cancel   context.CancelFunc   // Cancels the context that this reader runs in
	finished process.FinishedFunc // Tells the process manager we're finished and it can remove the associated process from the process table
}

var shaLineRegex = regexp.MustCompile("^([a-z0-9]{40})")

// NextPart returns next part of blame (sequential code lines with the same commit)
func (r *BlameReader) NextPart() (*BlamePart, error) {
	var blamePart *BlamePart

	reader := r.reader

	if r.lastSha != nil {
		blamePart = &BlamePart{*r.lastSha, make([]string, 0)}
	}

	var line []byte
	var isPrefix bool
	var err error

	for err != io.EOF {
		line, isPrefix, err = reader.ReadLine()
		if err != nil && err != io.EOF {
			return blamePart, err
		}

		if len(line) == 0 {
			// isPrefix will be false
			continue
		}

		lines := shaLineRegex.FindSubmatch(line)
		if lines != nil {
			sha1 := string(lines[1])

			if blamePart == nil {
				blamePart = &BlamePart{sha1, make([]string, 0)}
			}

			if blamePart.Sha != sha1 {
				r.lastSha = &sha1
				// need to munch to end of line...
				for isPrefix {
					_, isPrefix, err = reader.ReadLine()
					if err != nil && err != io.EOF {
						return blamePart, err
					}
				}
				return blamePart, nil
			}
		} else if line[0] == '\t' {
			code := line[1:]

			blamePart.Lines = append(blamePart.Lines, string(code))
		}

		// need to munch to end of line...
		for isPrefix {
			_, isPrefix, err = reader.ReadLine()
			if err != nil && err != io.EOF {
				return blamePart, err
			}
		}
	}

	r.lastSha = nil

	return blamePart, nil
}

// Close BlameReader - don't run NextPart after invoking that
func (r *BlameReader) Close() error {
	defer r.finished() // Only remove the process from the process table when the underlying command is closed
	r.cancel()         // However, first cancel our own context early

	_ = r.output.Close()

	if err := r.cmd.Wait(); err != nil {
		return fmt.Errorf("Wait: %w", err)
	}

	return nil
}

// CreateBlameReader creates reader for given repository, commit and file
func CreateBlameReader(ctx context.Context, repoPath, commitID, file string) (*BlameReader, error) {
	return createBlameReader(ctx, repoPath, GitExecutable, "blame", commitID, "--porcelain", "--", file)
}

func createBlameReader(ctx context.Context, dir string, command ...string) (*BlameReader, error) {
	// Here we use the provided context - this should be tied to the request performing the blame so that it does not hang around.
	ctx, cancel, finished := process.GetManager().AddContext(ctx, fmt.Sprintf("GetBlame [repo_path: %s]", dir))

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	process.SetSysProcAttribute(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		defer finished()
		return nil, fmt.Errorf("StdoutPipe: %w", err)
	}

	if err = cmd.Start(); err != nil {
		defer finished()
		_ = stdout.Close()
		return nil, fmt.Errorf("Start: %w", err)
	}

	reader := bufio.NewReader(stdout)

	return &BlameReader{
		cmd:      cmd,
		output:   stdout,
		reader:   reader,
		cancel:   cancel,
		finished: finished,
	}, nil
}
