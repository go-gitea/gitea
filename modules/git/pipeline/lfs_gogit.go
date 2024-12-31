// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package pipeline

import (
	"bufio"
	"io"
	"sort"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FindLFSFile finds commits that contain a provided pointer file hash
func FindLFSFile(repo *git.Repository, objectID git.ObjectID) ([]*LFSResult, error) {
	resultsMap := map[string]*LFSResult{}
	results := make([]*LFSResult, 0)

	basePath := repo.Path
	gogitRepo := repo.GoGitRepo()

	commitsIter, err := gogitRepo.Log(&gogit.LogOptions{
		Order: gogit.LogOrderCommitterTime,
		All:   true,
	})
	if err != nil {
		return nil, lfsError("failed to get GoGit CommitsIter", err)
	}

	err = commitsIter.ForEach(func(gitCommit *object.Commit) error {
		tree, err := gitCommit.Tree()
		if err != nil {
			return err
		}
		treeWalker := object.NewTreeWalker(tree, true, nil)
		defer treeWalker.Close()
		for {
			name, entry, err := treeWalker.Next()
			if err == io.EOF {
				break
			}
			if entry.Hash == plumbing.Hash(objectID.RawValue()) {
				parents := make([]git.ObjectID, len(gitCommit.ParentHashes))
				for i, parentCommitID := range gitCommit.ParentHashes {
					parents[i] = git.ParseGogitHash(parentCommitID)
				}

				result := LFSResult{
					Name:         name,
					SHA:          gitCommit.Hash.String(),
					Summary:      strings.Split(strings.TrimSpace(gitCommit.Message), "\n")[0],
					When:         gitCommit.Author.When,
					ParentHashes: parents,
				}
				resultsMap[gitCommit.Hash.String()+":"+name] = &result
			}
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, lfsError("failure in CommitIter.ForEach", err)
	}

	for _, result := range resultsMap {
		hasParent := false
		for _, parentHash := range result.ParentHashes {
			if _, hasParent = resultsMap[parentHash.String()+":"+result.Name]; hasParent {
				break
			}
		}
		if !hasParent {
			results = append(results, result)
		}
	}

	sort.Sort(lfsResultSlice(results))

	// Should really use a go-git function here but name-rev is not completed and recapitulating it is not simple
	shasToNameReader, shasToNameWriter := io.Pipe()
	nameRevStdinReader, nameRevStdinWriter := io.Pipe()
	errChan := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(nameRevStdinReader)
		i := 0
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 {
				continue
			}
			result := results[i]
			result.FullCommitName = line
			result.BranchName = strings.Split(line, "~")[0]
			i++
		}
	}()
	go NameRevStdin(repo.Ctx, shasToNameReader, nameRevStdinWriter, &wg, basePath)
	go func() {
		defer wg.Done()
		defer shasToNameWriter.Close()
		for _, result := range results {
			i := 0
			if i < len(result.SHA) {
				n, err := shasToNameWriter.Write([]byte(result.SHA)[i:])
				if err != nil {
					errChan <- err
					break
				}
				i += n
			}
			n := 0
			for n < 1 {
				n, err = shasToNameWriter.Write([]byte{'\n'})
				if err != nil {
					errChan <- err
					break
				}

			}

		}
	}()

	wg.Wait()

	select {
	case err, has := <-errChan:
		if has {
			return nil, lfsError("unable to obtain name for LFS files", err)
		}
	default:
	}

	return results, nil
}
