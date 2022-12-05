// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !gogit

package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// IsObjectExist returns true if given reference exists in the repository.
func (repo *Repository) IsObjectExist(name string) bool {
	if name == "" {
		return false
	}

	wr, rd, cancel := repo.CatFileBatchCheck(repo.Ctx)
	defer cancel()
	_, err := wr.Write([]byte(name + "\n"))
	if err != nil {
		log.Debug("Error writing to CatFileBatchCheck %v", err)
		return false
	}
	sha, _, _, err := ReadBatchLine(rd)
	return err == nil && bytes.HasPrefix(sha, []byte(strings.TrimSpace(name)))
}

// IsReferenceExist returns true if given reference exists in the repository.
func (repo *Repository) IsReferenceExist(name string) bool {
	if name == "" {
		return false
	}

	wr, rd, cancel := repo.CatFileBatchCheck(repo.Ctx)
	defer cancel()
	_, err := wr.Write([]byte(name + "\n"))
	if err != nil {
		log.Debug("Error writing to CatFileBatchCheck %v", err)
		return false
	}
	_, _, _, err = ReadBatchLine(rd)
	return err == nil
}

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if repo == nil || name == "" {
		return false
	}

	return repo.IsReferenceExist(BranchPrefix + name)
}

// GetBranchNames returns branches from the repository, skipping skip initial branches and
// returning at most limit branches, or all branches if limit is 0.
func (repo *Repository) GetBranchNames(skip, limit int) ([]string, int, error) {
	return callShowRef(repo.Ctx, repo.Path, BranchPrefix, "--heads", skip, limit)
}

// WalkReferences walks all the references from the repository
func WalkReferences(ctx context.Context, repoPath string, walkfn func(sha1, refname string) error) (int, error) {
	return walkShowRef(ctx, repoPath, "", 0, 0, walkfn)
}

// WalkReferences walks all the references from the repository
// refType should be empty, ObjectTag or ObjectBranch. All other values are equivalent to empty.
func (repo *Repository) WalkReferences(refType ObjectType, skip, limit int, walkfn func(sha1, refname string) error) (int, error) {
	var arg string
	switch refType {
	case ObjectTag:
		arg = "--tags"
	case ObjectBranch:
		arg = "--heads"
	default:
		arg = ""
	}

	return walkShowRef(repo.Ctx, repo.Path, arg, skip, limit, walkfn)
}

// callShowRef return refs, if limit = 0 it will not limit
func callShowRef(ctx context.Context, repoPath, prefix, arg string, skip, limit int) (branchNames []string, countAll int, err error) {
	countAll, err = walkShowRef(ctx, repoPath, arg, skip, limit, func(_, branchName string) error {
		branchName = strings.TrimPrefix(branchName, prefix)
		branchNames = append(branchNames, branchName)

		return nil
	})
	return
}

func walkShowRef(ctx context.Context, repoPath, arg string, skip, limit int, walkfn func(sha1, refname string) error) (countAll int, err error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go func() {
		stderrBuilder := &strings.Builder{}
		args := []string{"show-ref"}
		if arg != "" {
			args = append(args, arg)
		}
		err := NewCommand(ctx, args...).Run(&RunOpts{
			Dir:    repoPath,
			Stdout: stdoutWriter,
			Stderr: stderrBuilder,
		})
		if err != nil {
			if stderrBuilder.Len() == 0 {
				_ = stdoutWriter.Close()
				return
			}
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, stderrBuilder.String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	i := 0
	bufReader := bufio.NewReader(stdoutReader)
	for i < skip {
		_, isPrefix, err := bufReader.ReadLine()
		if err == io.EOF {
			return i, nil
		}
		if err != nil {
			return 0, err
		}
		if !isPrefix {
			i++
		}
	}
	for limit == 0 || i < skip+limit {
		// The output of show-ref is simply a list:
		// <sha> SP <ref> LF
		sha, err := bufReader.ReadString(' ')
		if err == io.EOF {
			return i, nil
		}
		if err != nil {
			return 0, err
		}

		branchName, err := bufReader.ReadString('\n')
		if err == io.EOF {
			// This shouldn't happen... but we'll tolerate it for the sake of peace
			return i, nil
		}
		if err != nil {
			return i, err
		}

		if len(branchName) > 0 {
			branchName = branchName[:len(branchName)-1]
		}

		if len(sha) > 0 {
			sha = sha[:len(sha)-1]
		}

		err = walkfn(sha, branchName)
		if err != nil {
			return i, err
		}
		i++
	}
	// count all refs
	for limit != 0 {
		_, isPrefix, err := bufReader.ReadLine()
		if err == io.EOF {
			return i, nil
		}
		if err != nil {
			return 0, err
		}
		if !isPrefix {
			i++
		}
	}
	return i, nil
}

// GetRefsBySha returns all references filtered with prefix that belong to a sha commit hash
func (repo *Repository) GetRefsBySha(sha, prefix string) ([]string, error) {
	var revList []string
	_, err := walkShowRef(repo.Ctx, repo.Path, "", 0, 0, func(walkSha, refname string) error {
		if walkSha == sha && strings.HasPrefix(refname, prefix) {
			revList = append(revList, refname)
		}
		return nil
	})
	return revList, err
}
