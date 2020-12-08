// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bufio"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

// GetRefsFiltered returns all references of the repository that matches patterm exactly or starting with.
func (repo *Repository) GetRefsFiltered(pattern string) ([]service.Reference, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go common.PipeCommand(git.NewCommand("for-each-ref"), repo.Path(), stdoutWriter, nil)

	refs := make([]service.Reference, 0)
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
		if strings.HasPrefix(refName, "/refs/remotes/") || refName == "/refs/stash" {
			continue
		}

		if pattern == "" || strings.HasPrefix(refName, pattern) {
			r := common.NewReference(
				refName,
				StringHash(sha),
				service.ObjectType(typ),
				repo,
			)
			refs = append(refs, r)
		}
	}

	return refs, nil
}

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]service.Reference, error) {
	return repo.GetRefsFiltered("")
}
