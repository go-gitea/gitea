// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
	"context"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
)

// IsObjectExist returns true if the given object exists in the repository.
// FIXME: this function doesn't seem right, it is only used by GarbageCollectLFSMetaObjectsForRepo
func (repo *Repository) IsObjectExist(name string) bool {
	if name == "" {
		return false
	}

	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		log.Debug("Error opening CatFileBatch %v", err)
		return false
	}
	defer cancel()
	info, err := batch.QueryInfo(name)
	if err != nil {
		log.Debug("Error checking object info %v", err)
		return false
	}
	return strings.HasPrefix(info.ID, name) // FIXME: this logic doesn't seem right, why "HasPrefix"
}

// IsReferenceExist returns true if given reference exists in the repository.
func (repo *Repository) IsReferenceExist(name string) bool {
	if name == "" {
		return false
	}

	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		log.Error("Error opening CatFileBatch %v", err)
		return false
	}
	defer cancel()
	_, err = batch.QueryInfo(name)
	return err == nil
}

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if repo == nil || name == "" {
		return false
	}

	return repo.IsReferenceExist(BranchPrefix + name)
}

// GetBranchNames returns branches from the repository, skipping "skip" initial branches and
// returning at most "limit" branches, or all branches if "limit" is 0.
func (repo *Repository) GetBranchNames(skip, limit int) ([]string, int, error) {
	return callShowRef(repo.Ctx, repo.Path, BranchPrefix, gitcmd.TrustedCmdArgs{BranchPrefix, "--sort=-committerdate"}, skip, limit)
}

// WalkReferences walks all the references from the repository
// refType should be empty, ObjectTag or ObjectBranch. All other values are equivalent to empty.
func (repo *Repository) WalkReferences(refType ObjectType, skip, limit int, walkfn func(sha1, refname string) error) (int, error) {
	var args gitcmd.TrustedCmdArgs
	switch refType {
	case ObjectTag:
		args = gitcmd.TrustedCmdArgs{TagPrefix, "--sort=-taggerdate"}
	case ObjectBranch:
		args = gitcmd.TrustedCmdArgs{BranchPrefix, "--sort=-committerdate"}
	}

	return WalkShowRef(repo.Ctx, repo.Path, args, skip, limit, walkfn)
}

// callShowRef return refs, if limit = 0 it will not limit
func callShowRef(ctx context.Context, repoPath, trimPrefix string, extraArgs gitcmd.TrustedCmdArgs, skip, limit int) (branchNames []string, countAll int, err error) {
	countAll, err = WalkShowRef(ctx, repoPath, extraArgs, skip, limit, func(_, branchName string) error {
		branchName = strings.TrimPrefix(branchName, trimPrefix)
		branchNames = append(branchNames, branchName)

		return nil
	})
	return branchNames, countAll, err
}

func WalkShowRef(ctx context.Context, repoPath string, extraArgs gitcmd.TrustedCmdArgs, skip, limit int, walkfn func(sha1, refname string) error) (countAll int, err error) {
	i := 0
	args := gitcmd.TrustedCmdArgs{"for-each-ref", "--format=%(objectname) %(refname)"}
	args = append(args, extraArgs...)
	cmd := gitcmd.NewCommand(args...)
	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()
	cmd.WithDir(repoPath).
		WithPipelineFunc(func(gitcmd.Context) error {
			bufReader := bufio.NewReader(stdoutReader)
			for i < skip {
				_, isPrefix, err := bufReader.ReadLine()
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
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
					return nil
				}
				if err != nil {
					return err
				}

				branchName, err := bufReader.ReadString('\n')
				if err == io.EOF {
					// This shouldn't happen... but we'll tolerate it for the sake of peace
					return nil
				}
				if err != nil {
					return err
				}

				if len(branchName) > 0 {
					branchName = branchName[:len(branchName)-1]
				}

				if len(sha) > 0 {
					sha = sha[:len(sha)-1]
				}

				err = walkfn(sha, branchName)
				if err != nil {
					return err
				}
				i++
			}
			// count all refs
			for limit != 0 {
				_, isPrefix, err := bufReader.ReadLine()
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
				}
				if !isPrefix {
					i++
				}
			}
			return nil
		})
	err = cmd.RunWithStderr(ctx)
	if errPipeline, ok := gitcmd.UnwrapPipelineError(err); ok {
		return i, errPipeline // keep the old behavior: return pipeline error directly
	}
	return i, err
}

// GetRefsBySha returns all references filtered with prefix that belong to a sha commit hash
func (repo *Repository) GetRefsBySha(sha, prefix string) ([]string, error) {
	var revList []string
	_, err := WalkShowRef(repo.Ctx, repo.Path, nil, 0, 0, func(walkSha, refname string) error {
		if walkSha == sha && strings.HasPrefix(refname, prefix) {
			revList = append(revList, refname)
		}
		return nil
	})
	return revList, err
}
