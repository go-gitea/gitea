// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build gogit

package pipeline

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/git"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// LFSResult represents commits found using a provided pointer file hash
type LFSResult struct {
	Name           string
	SHA            string
	Summary        string
	When           time.Time
	ParentHashes   []git.SHA1
	BranchName     string
	FullCommitName string
}

type lfsResultSlice []*LFSResult

func (a lfsResultSlice) Len() int           { return len(a) }
func (a lfsResultSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a lfsResultSlice) Less(i, j int) bool { return a[j].When.After(a[i].When) }

// FindLFSFile finds commits that contain a provided pointer file hash
func FindLFSFile(repo *git.Repository, hash git.SHA1) ([]*LFSResult, error) {
	resultsMap := map[string]*LFSResult{}
	results := make([]*LFSResult, 0)

	basePath := repo.Path
	gogitRepo := repo.GoGitRepo()

	commitsIter, err := gogitRepo.Log(&gogit.LogOptions{
		Order: gogit.LogOrderCommitterTime,
		All:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get GoGit CommitsIter. Error: %w", err)
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
			if entry.Hash == hash {
				result := LFSResult{
					Name:         name,
					SHA:          gitCommit.Hash.String(),
					Summary:      strings.Split(strings.TrimSpace(gitCommit.Message), "\n")[0],
					When:         gitCommit.Author.When,
					ParentHashes: gitCommit.ParentHashes,
				}
				resultsMap[gitCommit.Hash.String()+":"+name] = &result
			}
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("Failure in CommitIter.ForEach: %w", err)
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
			return nil, fmt.Errorf("Unable to obtain name for LFS files. Error: %w", err)
		}
	default:
	}

	return results, nil
}
