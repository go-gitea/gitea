// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package pipeline

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"io"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
)

// FindLFSFile finds commits that contain a provided pointer file hash
func FindLFSFile(repo *git.Repository, objectID git.ObjectID) (results []*LFSResult, _ error) {
	cmd := gitcmd.NewCommand("rev-list", "--all")
	revListReader, revListReaderClose := cmd.MakeStdoutPipe()
	defer revListReaderClose()
	err := cmd.WithDir(repo.Path).
		WithPipelineFunc(func(context gitcmd.Context) (err error) {
			results, err = findLFSFileFunc(repo, objectID, revListReader)
			return err
		}).RunWithStderr(repo.Ctx)
	return results, err
}

func findLFSFileFunc(repo *git.Repository, objectID git.ObjectID, revListReader io.Reader) ([]*LFSResult, error) {
	resultsMap := map[string]*LFSResult{}
	results := make([]*LFSResult, 0)
	// Next feed the commits in order into cat-file --batch, followed by their trees and sub trees as necessary.
	// so let's create a batch stdin and stdout
	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	// We'll use a scanner for the revList because it's simpler than a bufio.Reader
	scan := bufio.NewScanner(revListReader)
	trees := []string{}
	paths := []string{}

	fnameBuf := make([]byte, 4096)
	modeBuf := make([]byte, 40)
	workingShaBuf := make([]byte, objectID.Type().FullLength()/2)

	for scan.Scan() {
		// Get the next commit ID
		commitID := scan.Text()

		// push the commit to the cat-file --batch process
		info, batchReader, err := batch.QueryContent(commitID)
		if err != nil {
			return nil, err
		}

		var curCommit *git.Commit
		curPath := ""

	commitReadingLoop:
		for {
			switch info.Type {
			case "tag":
				// This shouldn't happen but if it does well just get the commit and try again
				id, err := git.ReadTagObjectID(batchReader, info.Size)
				if err != nil {
					return nil, err
				}
				if info, batchReader, err = batch.QueryContent(id); err != nil {
					return nil, err
				}
				continue
			case "commit":
				// Read in the commit to get its tree and in case this is one of the last used commits
				curCommit, err = git.CommitFromReader(repo, git.MustIDFromString(commitID), io.LimitReader(batchReader, info.Size))
				if err != nil {
					return nil, err
				}
				if _, err := batchReader.Discard(1); err != nil {
					return nil, err
				}

				if info, _, err = batch.QueryContent(curCommit.Tree.ID.String()); err != nil {
					return nil, err
				}
				curPath = ""
			case "tree":
				var n int64
				for n < info.Size {
					mode, fname, binObjectID, count, err := git.ParseCatFileTreeLine(objectID.Type(), batchReader, modeBuf, fnameBuf, workingShaBuf)
					if err != nil {
						return nil, err
					}
					n += int64(count)
					if bytes.Equal(binObjectID, objectID.RawValue()) {
						result := LFSResult{
							Name:         curPath + string(fname),
							SHA:          curCommit.ID.String(),
							Summary:      strings.Split(strings.TrimSpace(curCommit.CommitMessage), "\n")[0],
							When:         curCommit.Author.When,
							ParentHashes: curCommit.Parents,
						}
						resultsMap[curCommit.ID.String()+":"+curPath+string(fname)] = &result
					} else if string(mode) == git.EntryModeTree.String() {
						trees = append(trees, hex.EncodeToString(binObjectID))
						paths = append(paths, curPath+string(fname)+"/")
					}
				}
				if _, err := batchReader.Discard(1); err != nil {
					return nil, err
				}
				if len(trees) > 0 {
					info, _, err = batch.QueryContent(trees[len(trees)-1])
					if err != nil {
						return nil, err
					}
					curPath = paths[len(paths)-1]
					trees = trees[:len(trees)-1]
					paths = paths[:len(paths)-1]
				} else {
					break commitReadingLoop
				}
			default:
				if err := git.DiscardFull(batchReader, info.Size+1); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := scan.Err(); err != nil {
		return nil, err
	}

	for _, result := range resultsMap {
		hasParent := false
		for _, parentID := range result.ParentHashes {
			if _, hasParent = resultsMap[parentID.String()+":"+result.Name]; hasParent {
				break
			}
		}
		if !hasParent {
			results = append(results, result)
		}
	}

	sort.Sort(lfsResultSlice(results))
	err = fillResultNameRev(repo.Ctx, repo.Path, results)
	return results, err
}
