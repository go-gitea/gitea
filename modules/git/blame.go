// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"

	"code.gitea.io/gitea/modules/util"
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
	ignoreRevsFile *os.File
}

func (r *BlameReader) UsesIgnoreRevs() bool {
	return r.ignoreRevsFile != nil
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
	if r.ignoreRevsFile != nil {
		_ = util.Remove(r.ignoreRevsFile.Name())
	}
	return err
}

// CreateBlameReader creates reader for given repository, commit and file
func CreateBlameReader(ctx context.Context, repoPath string, commit *Commit, file string, bypassBlameIgnore bool) (*BlameReader, error) {
	cmd := NewCommandContextNoGlobals(ctx, "blame", "--porcelain")

	var ignoreRevsFile *os.File
	if !bypassBlameIgnore {
		ignoreRevsFile = tryCreateBlameIgnoreRevsFile(commit)
		if ignoreRevsFile != nil {
			cmd.AddOptionValues("--ignore-revs-file", ignoreRevsFile.Name())
		}
	}

	cmd.AddDynamicArguments(commit.ID.String()).
		AddDashesAndList(file).
		SetDescription(fmt.Sprintf("GetBlame [repo_path: %s]", repoPath))
	reader, stdout, err := os.Pipe()
	if err != nil {
		if ignoreRevsFile != nil {
			_ = util.Remove(ignoreRevsFile.Name())
		}
		return nil, err
	}

	done := make(chan error, 1)

	go func(cmd *Command, dir string, stdout io.WriteCloser, done chan error) {
		if err := cmd.Run(&RunOpts{
			UseContextTimeout: true,
			Dir:               dir,
			Stdout:            stdout,
			Stderr:            os.Stderr,
		}); err == nil {
			stdout.Close()
		}
		done <- err
	}(cmd, repoPath, stdout, done)

	bufferedReader := bufio.NewReader(reader)

	return &BlameReader{
		cmd:            cmd,
		output:         stdout,
		reader:         reader,
		bufferedReader: bufferedReader,
		done:           done,
		ignoreRevsFile: ignoreRevsFile,
	}, nil
}

func tryCreateBlameIgnoreRevsFile(commit *Commit) *os.File {
	entry, err := commit.GetTreeEntryByPath(".git-blame-ignore-revs")
	if err != nil {
		return nil
	}

	r, err := entry.Blob().DataAsync()
	if err != nil {
		return nil
	}
	defer r.Close()

	f, err := os.CreateTemp("", "gitea_git-blame-ignore-revs")
	if err != nil {
		return nil
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	if err != nil {
		defer func() {
			_ = util.Remove(f.Name())
		}()
		return nil
	}

	return f
}
