// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"bytes"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

// HashObject takes a reader and returns SHA1 hash for that reader
func (repo *Repository) HashObject(reader io.Reader) (service.Hash, error) {
	idStr, err := repo.hashObject(reader)
	if err != nil {
		return StringHash(""), err
	}
	return StringHash(idStr), nil
}

func (repo *Repository) hashObject(reader io.Reader) (string, error) {
	cmd := git.NewCommand("hash-object", "-w", "--stdin")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	err := cmd.RunInDirFullPipeline(repo.Path(), stdout, stderr, reader)

	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// GetRefType gets the type of the ref based on the string
func (repo *Repository) GetRefType(ref string) service.ObjectType {
	// FIXME: this appears somewhat inefficient

	if repo.IsTagExist(ref) {
		return service.ObjectTag
	} else if repo.IsBranchExist(ref) {
		return service.ObjectBranch
	} else if repo.IsCommitExist(ref) {
		return service.ObjectCommit
	} else if _, err := repo.GetBlob(ref); err == nil {
		return service.ObjectBlob
	}
	return service.ObjectType("invalid")
}
