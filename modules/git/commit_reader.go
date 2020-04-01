// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
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
	// Split by '\n' but include the '\n'
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			// We have a full newline-terminated line.
			return i + 1, data[0 : i+1], nil
		}
		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			return len(data), data, nil
		}
		// Request more data.
		return 0, nil, nil
	})

	for scanner.Scan() {
		line := scanner.Bytes()
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
				commit.Tree = *NewTree(gitRepo, plumbing.NewHash(string(data)))
				_, _ = payloadSB.Write(line)
			case "parent":
				commit.Parents = append(commit.Parents, plumbing.NewHash(string(data)))
				_, _ = payloadSB.Write(line)
			case "author":
				commit.Author = &Signature{}
				commit.Author.Decode(data)
				_, _ = payloadSB.Write(line)
			case "committer":
				commit.Committer = &Signature{}
				commit.Committer.Decode(data)
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
