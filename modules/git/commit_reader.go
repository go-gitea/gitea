// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

const (
	commitHeaderGpgsig       = "gpgsig"
	commitHeaderGpgsigSha256 = "gpgsig-sha256"
)

func assignCommitFields(gitRepo *Repository, commit *Commit, headerKey string, headerValue []byte) error {
	if len(headerValue) > 0 && headerValue[len(headerValue)-1] == '\n' {
		headerValue = headerValue[:len(headerValue)-1] // remove trailing newline
	}
	switch headerKey {
	case "tree":
		objID, err := NewIDFromString(string(headerValue))
		if err != nil {
			return fmt.Errorf("invalid tree ID %q: %w", string(headerValue), err)
		}
		commit.Tree = *NewTree(gitRepo, objID)
	case "parent":
		objID, err := NewIDFromString(string(headerValue))
		if err != nil {
			return fmt.Errorf("invalid parent ID %q: %w", string(headerValue), err)
		}
		commit.Parents = append(commit.Parents, objID)
	case "author":
		commit.Author.Decode(headerValue)
	case "committer":
		commit.Committer.Decode(headerValue)
	case commitHeaderGpgsig, commitHeaderGpgsigSha256:
		// if there are duplicate "gpgsig" and "gpgsig-sha256" headers, then the signature must have already been invalid
		// so we don't need to handle duplicate headers here
		commit.Signature = &CommitSignature{Signature: string(headerValue)}
	}
	return nil
}

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

	bufReader := bufio.NewReader(reader)
	inHeader := true
	var payloadSB, messageSB bytes.Buffer
	var headerKey string
	var headerValue []byte
	for {
		line, err := bufReader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("unable to read commit %q: %w", objectID.String(), err)
		}
		if len(line) == 0 {
			break
		}

		if inHeader {
			inHeader = !(len(line) == 1 && line[0] == '\n') // still in header if line is not just a newline
			k, v, _ := bytes.Cut(line, []byte{' '})
			if len(k) != 0 || !inHeader {
				if headerKey != "" {
					if err = assignCommitFields(gitRepo, commit, headerKey, headerValue); err != nil {
						return nil, fmt.Errorf("unable to parse commit %q: %w", objectID.String(), err)
					}
				}
				headerKey = string(k) // it also resets the headerValue to empty string if not inHeader
				headerValue = v
			} else {
				headerValue = append(headerValue, v...)
			}
			if headerKey != commitHeaderGpgsig && headerKey != commitHeaderGpgsigSha256 {
				_, _ = payloadSB.Write(line)
			}
		} else {
			_, _ = messageSB.Write(line)
			_, _ = payloadSB.Write(line)
		}
	}

	commit.CommitMessage = messageSB.String()
	if commit.Signature != nil {
		commit.Signature.Payload = payloadSB.String()
	}
	return commit, nil
}
