// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"

	"code.gitea.io/gitea/modules/log"
)

// BlamePart represents block of blame - continuous lines with one sha
type BlamePart struct {
	Sha   string
	Lines []string
}

// BlameReader returns part of file blame one by one
type BlameReader struct {
	cmd            *Command
	output         io.WriteCloser
	reader         io.ReadCloser
	bufferedReader *bufio.Reader
	done           chan error
	lastSha        *string
}

var shaLineRegex = regexp.MustCompile("^([a-z0-9]{40})")

// NextPart returns next part of blame (sequential code lines with the same commit)
func (r *BlameReader) NextPart() (*BlamePart, error) {
	var blamePart *BlamePart

	if r.lastSha != nil {
		blamePart = &BlamePart{*r.lastSha, make([]string, 0)}
	}

	var line []byte
	var isPrefix bool
	var err error

	for err != io.EOF {
		line, isPrefix, err = r.bufferedReader.ReadLine()
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
					_, isPrefix, err = r.bufferedReader.ReadLine()
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
			_, isPrefix, err = r.bufferedReader.ReadLine()
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
	err := <-r.done
	r.bufferedReader = nil
	_ = r.reader.Close()
	_ = r.output.Close()
	return err
}

// CreateBlameReader creates reader for given repository, commit and file
func CreateBlameReader(ctx context.Context, repoPath, commitID, file string) (*BlameReader, error) {
	cmd := NewCommandContextNoGlobals(ctx, "blame", "--porcelain").
		AddDynamicArguments(commitID).
		AddDashesAndList(file).
		SetDescription(fmt.Sprintf("GetBlame [repo_path: %s]", repoPath))
	reader, stdout, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	done := make(chan error, 1)

	go func(cmd *Command, dir string, stdout io.WriteCloser, done chan error) {
		stderr := bytes.Buffer{}
		// TODO: it doesn't work for directories (the directories shouldn't be "blamed"), and the "err" should be returned by "Read" but not by "Close"
		err := cmd.Run(&RunOpts{
			UseContextTimeout: true,
			Dir:               dir,
			Stdout:            stdout,
			Stderr:            &stderr,
		})
		done <- err
		_ = stdout.Close()
		if err != nil {
			log.Error("Error running git blame (dir: %v): %v, stderr: %v", repoPath, err, stderr.String())
		}
	}(cmd, repoPath, stdout, done)

	bufferedReader := bufio.NewReader(reader)

	return &BlameReader{
		cmd:            cmd,
		output:         stdout,
		reader:         reader,
		bufferedReader: bufferedReader,
		done:           done,
	}, nil
}
