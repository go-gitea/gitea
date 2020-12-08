// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bufio"
	"bytes"
	"container/list"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

// CommitFromReader will generate a Commit from a provided reader
// We need this to interpret commits from cat-file or cat-file --batch
//
// If used as part of a cat-file --batch stream you need to limit the reader to the correct size
func CommitFromReader(repo service.Repository, sha service.Hash, reader io.Reader) (*Commit, error) {
	commit := &Commit{
		Object: &Object{
			hash: sha,
			repo: repo,
		},
	}

	payloadSB := new(strings.Builder)
	signatureSB := new(strings.Builder)
	messageSB := new(strings.Builder)
	message := false
	pgpsig := false

	bufReader, ok := reader.(*bufio.Reader)
	if !ok {
		bufReader = bufio.NewReader(reader)
	}

readLoop:
	for {
		line, err := bufReader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break readLoop
			}
			return nil, err
		}
		if pgpsig {
			if len(line) > 0 && line[0] == ' ' {
				_, _ = signatureSB.Write(line[1:])
				continue
			} else {
				pgpsig = false
			}
		}

		if !message {
			// This is probably not correct but is copied from go-gits interpretation...
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) == 0 {
				message = true
				_, _ = payloadSB.Write(line)
				continue
			}

			split := bytes.SplitN(trimmed, []byte{' '}, 2)
			var data []byte
			if len(split) > 1 {
				data = split[1]
			}

			switch string(split[0]) {
			case "tree":
				commit.ptree = &Tree{
					Object: Object{
						hash: StringHash(string(data)),
						repo: repo,
					},
				}
				_, _ = payloadSB.Write(line)
			case "parent":
				commit.parents = append(commit.parents, StringHash(string(data)))
				_, _ = payloadSB.Write(line)
			case "author":
				commit.author = &service.Signature{}
				commit.author.Decode(data)
				_, _ = payloadSB.Write(line)
			case "committer":
				commit.committer = &service.Signature{}
				commit.committer.Decode(data)
				_, _ = payloadSB.Write(line)
			case "gpgsig":
				_, _ = signatureSB.Write(data)
				_ = signatureSB.WriteByte('\n')
				pgpsig = true
			}
		} else {
			_, _ = messageSB.Write(line)
		}
	}
	commit.message = messageSB.String()
	_, _ = payloadSB.WriteString(commit.message)
	commit.signature = &service.GPGSignature{
		Signature: signatureSB.String(),
		Payload:   payloadSB.String(),
	}
	if len(commit.signature.Signature) == 0 {
		commit.signature = nil
	}

	return commit, nil
}

// BatchReadCommits converts a stdin to commits list
func BatchReadCommits(repo service.Repository, stdin io.Reader) (*list.List, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	go common.PipeCommand(
		git.NewCommand("cat-file", "--batch"),
		repo.Path(),
		stdoutWriter,
		stdin)

	bufReader := bufio.NewReader(stdoutReader)

	commits := &list.List{}

	var err error
loop:
	for err == nil {
		var size int64
		var commit *Commit
		var sha []byte
		sha, _, size, err = ReadBatchLine(bufReader)
		if err != nil {
			break loop
		}
		commit, err = CommitFromReader(repo, StringHash(string(sha)), io.LimitReader(bufReader, size))
		commits.PushBack(commit)
		if err != nil {
			break loop
		}
	}

	if err != nil && err != io.EOF {
		return nil, err
	}

	return commits, nil
}
