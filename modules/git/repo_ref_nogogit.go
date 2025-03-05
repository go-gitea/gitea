// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
	"io"
	"strings"
)

// GetRefsFiltered returns all references of the repository that matches patterm exactly or starting with.
func (repo *Repository) GetRefsFiltered(pattern string) ([]*Reference, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go func() {
		stderrBuilder := &strings.Builder{}
		err := NewCommand("for-each-ref").Run(repo.Ctx, &RunOpts{
			Dir:    repo.Path,
			Stdout: stdoutWriter,
			Stderr: stderrBuilder,
		})
		if err != nil {
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, stderrBuilder.String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	refs := make([]*Reference, 0)
	bufReader := bufio.NewReader(stdoutReader)
	for {
		// The output of for-each-ref is simply a list:
		// <sha> SP <type> TAB <ref> LF
		sha, err := bufReader.ReadString(' ')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		sha = sha[:len(sha)-1]

		typ, err := bufReader.ReadString('\t')
		if err == io.EOF {
			// This should not happen, but we'll tolerate it
			break
		}
		if err != nil {
			return nil, err
		}
		typ = typ[:len(typ)-1]

		refName, err := bufReader.ReadString('\n')
		if err == io.EOF {
			// This should not happen, but we'll tolerate it
			break
		}
		if err != nil {
			return nil, err
		}
		refName = refName[:len(refName)-1]

		// refName cannot be HEAD but can be remotes or stash
		if strings.HasPrefix(refName, RemotePrefix) || refName == "/refs/stash" {
			continue
		}

		if pattern == "" || strings.HasPrefix(refName, pattern) {
			r := &Reference{
				Name:   refName,
				Object: MustIDFromString(sha),
				Type:   typ,
				repo:   repo,
			}
			refs = append(refs, r)
		}
	}

	return refs, nil
}
