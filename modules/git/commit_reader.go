// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// CommitFromReader will generate a Commit from a provided reader
// We need this to interpret commits from cat-file or cat-file --batch
//
// If used as part of a cat-file --batch stream you need to limit the reader to the correct size
func CommitFromReader(gitRepo *Repository, objectID ObjectID, reader io.Reader) (*Commit, error) {
	commit := &Commit{
		ID:        objectID,
		Author:    &Signature{},
		Committer: &Signature{},
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
				if message {
					_, _ = messageSB.Write(line)
				}
				_, _ = payloadSB.Write(line)
				break readLoop
			}
			return nil, err
		}
		if pgpsig {
			if len(line) > 0 && line[0] == ' ' {
				_, _ = signatureSB.Write(line[1:])
				continue
			}
			pgpsig = false
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
				commit.Tree = *NewTree(gitRepo, MustIDFromString(string(data)))
				_, _ = payloadSB.Write(line)
			case "parent":
				commit.Parents = append(commit.Parents, MustIDFromString(string(data)))
				_, _ = payloadSB.Write(line)
			case "author":
				commit.Author = &Signature{}
				commit.Author.Decode(data)
				_, _ = payloadSB.Write(line)
			case "committer":
				commit.Committer = &Signature{}
				commit.Committer.Decode(data)
				_, _ = payloadSB.Write(line)
			case "encoding":
				_, _ = payloadSB.Write(line)
			case "gpgsig":
				fallthrough
			case "gpgsig-sha256": // FIXME: no intertop, so only 1 exists at present.
				_, _ = signatureSB.Write(data)
				_ = signatureSB.WriteByte('\n')
				pgpsig = true
			}
		} else {
			_, _ = messageSB.Write(line)
			_, _ = payloadSB.Write(line)
		}
	}
	commit.CommitMessage = messageSB.String()
	commit.Signature = &CommitSignature{
		Signature: signatureSB.String(),
		Payload:   payloadSB.String(),
	}
	if len(commit.Signature.Signature) == 0 {
		commit.Signature = nil
	}

	return commit, nil
}
