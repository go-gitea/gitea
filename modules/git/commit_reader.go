// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

// CommitFromReader will generate a Commit from a provided reader
// We will need this to interpret commits from cat-file
func CommitFromReader(gitRepo *Repository, sha plumbing.Hash, reader io.Reader) (*Commit, error) {
	commit := &Commit{
		ID: sha,
	}

	payloadSB := new(strings.Builder)
	signatureSB := new(strings.Builder)
	messageSB := new(strings.Builder)
	message := false
	pgpsig := false

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		if pgpsig {
			if len(line) > 0 && line[0] == ' ' {
				line = bytes.TrimLeft(line, " ")
				_, _ = signatureSB.Write(line)
				_ = signatureSB.WriteByte('\n')
				continue
			} else {
				pgpsig = false
			}
		}

		if !message {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) == 0 {
				message = true
				_, _ = payloadSB.WriteString("\n")
				continue
			}

			split := bytes.SplitN(trimmed, []byte{' '}, 2)

			switch string(split[0]) {
			case "tree":
				commit.Tree = *NewTree(gitRepo, plumbing.NewHash(string(split[1])))
				_, _ = payloadSB.Write(line)
				_ = payloadSB.WriteByte('\n')
			case "parent":
				commit.Parents = append(commit.Parents, plumbing.NewHash(string(split[1])))
				_, _ = payloadSB.Write(line)
				_ = payloadSB.WriteByte('\n')
			case "author":
				commit.Author = &Signature{}
				commit.Author.Decode(split[1])
				_, _ = payloadSB.Write(line)
				_ = payloadSB.WriteByte('\n')
			case "committer":
				commit.Committer = &Signature{}
				commit.Committer.Decode(split[1])
				_, _ = payloadSB.Write(line)
				_ = payloadSB.WriteByte('\n')
			case "gpgsig":
				_, _ = signatureSB.Write(split[1])
				_ = signatureSB.WriteByte('\n')
				pgpsig = true
			}
		} else {
			_, _ = messageSB.Write(line)
			_ = messageSB.WriteByte('\n')
		}
	}
	commit.CommitMessage = messageSB.String()
	_, _ = payloadSB.WriteString(commit.CommitMessage)
	commit.Signature = &CommitGPGSignature{
		Signature: signatureSB.String(),
		Payload:   payloadSB.String(),
	}
	if len(commit.Signature.Signature) == 0 {
		commit.Signature = nil
	}

	return commit, scanner.Err()
}
