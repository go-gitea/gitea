// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// parameters for searching for commit infos. If the untargeted search has
	// not found any entries in the past 5 commits, and 12 or fewer entries
	// remain, then we'll just let the targeted-searching threads finish off,
	// and stop the untargeted search to not interfere.
	deferToTargetedSearchColdStreak          = 5
	deferToTargetedSearchNumRemainingEntries = 12
)

// getCommitsInfoState shared state while getting commit info for entries
type getCommitsInfoState struct {
	lock sync.Mutex
	/* read-only fields, can be read without the mutex */
	// entries and entryPaths are read-only after initialization, so they can
	// safely be read without the mutex
	entries []*TreeEntry
	// set of filepaths to get info for
	entryPaths map[string]struct{}
	treePath   string
	headCommit *Commit

	/* mutable fields, must hold mutex to read or write */
	// map from filepath to commit
	commits map[string]*Commit
	// set of filepaths that have been or are being searched for in a target search
	targetedPaths map[string]struct{}
}

func (state *getCommitsInfoState) numRemainingEntries() int {
	state.lock.Lock()
	defer state.lock.Unlock()
	return len(state.entries) - len(state.commits)
}

// getTargetEntryPath Returns the next path for a targeted-searching thread to
// search for, or returns the empty string if nothing left to search for
func (state *getCommitsInfoState) getTargetedEntryPath() string {
	var targetedEntryPath string
	state.lock.Lock()
	defer state.lock.Unlock()
	for _, entry := range state.entries {
		entryPath := path.Join(state.treePath, entry.Name())
		if _, ok := state.commits[entryPath]; ok {
			continue
		} else if _, ok = state.targetedPaths[entryPath]; ok {
			continue
		}
		targetedEntryPath = entryPath
		state.targetedPaths[entryPath] = struct{}{}
		break
	}
	return targetedEntryPath
}

// repeatedly perform targeted searches for unpopulated entries
func targetedSearch(state *getCommitsInfoState, done chan error, cache LastCommitCache) {
	for {
		entryPath := state.getTargetedEntryPath()
		if len(entryPath) == 0 {
			done <- nil
			return
		}
		if cache != nil {
			commit, err := cache.Get(state.headCommit.repo.Path, state.headCommit.ID.String(), entryPath)
			if err == nil && commit != nil {
				state.update(entryPath, commit)
				continue
			}
		}
		command := NewCommand("rev-list", "-1", state.headCommit.ID.String(), "--", entryPath)
		output, err := command.RunInDir(state.headCommit.repo.Path)
		if err != nil {
			done <- err
			return
		}
		id, err := NewIDFromString(strings.TrimSpace(output))
		if err != nil {
			done <- err
			return
		}
		commit, err := state.headCommit.repo.getCommit(id)
		if err != nil {
			done <- err
			return
		}
		state.update(entryPath, commit)
		if cache != nil {
			cache.Put(state.headCommit.repo.Path, state.headCommit.ID.String(), entryPath, commit)
		}
	}
}

func initGetCommitInfoState(entries Entries, headCommit *Commit, treePath string) *getCommitsInfoState {
	entryPaths := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		entryPaths[path.Join(treePath, entry.Name())] = struct{}{}
	}
	if treePath = path.Clean(treePath); treePath == "." {
		treePath = ""
	}
	return &getCommitsInfoState{
		entries:       entries,
		entryPaths:    entryPaths,
		commits:       make(map[string]*Commit, len(entries)),
		targetedPaths: make(map[string]struct{}, len(entries)),
		treePath:      treePath,
		headCommit:    headCommit,
	}
}

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(commit *Commit, treePath string, cache LastCommitCache) ([][]interface{}, error) {
	state := initGetCommitInfoState(tes, commit, treePath)
	if err := getCommitsInfo(state, cache); err != nil {
		return nil, err
	}
	if len(state.commits) < len(state.entryPaths) {
		return nil, fmt.Errorf("could not find commits for all entries")
	}

	commitsInfo := make([][]interface{}, len(tes))
	for i, entry := range tes {
		commit, ok := state.commits[path.Join(treePath, entry.Name())]
		if !ok {
			return nil, fmt.Errorf("could not find commit for %s", entry.Name())
		}
		switch entry.Type {
		case ObjectCommit:
			subModuleURL := ""
			if subModule, err := state.headCommit.GetSubModule(entry.Name()); err != nil {
				return nil, err
			} else if subModule != nil {
				subModuleURL = subModule.URL
			}
			subModuleFile := NewSubModuleFile(commit, subModuleURL, entry.ID.String())
			commitsInfo[i] = []interface{}{entry, subModuleFile}
		default:
			commitsInfo[i] = []interface{}{entry, commit}
		}
	}
	return commitsInfo, nil
}

func (state *getCommitsInfoState) cleanEntryPath(rawEntryPath string) (string, error) {
	if rawEntryPath[0] == '"' {
		var err error
		rawEntryPath, err = strconv.Unquote(rawEntryPath)
		if err != nil {
			return rawEntryPath, err
		}
	}
	var entryNameStartIndex int
	if len(state.treePath) > 0 {
		entryNameStartIndex = len(state.treePath) + 1
	}

	if index := strings.IndexByte(rawEntryPath[entryNameStartIndex:], '/'); index >= 0 {
		return rawEntryPath[:entryNameStartIndex+index], nil
	}
	return rawEntryPath, nil
}

// update report that the given path was last modified by the given commit.
// Returns whether state.commits was updated
func (state *getCommitsInfoState) update(entryPath string, commit *Commit) bool {
	if _, ok := state.entryPaths[entryPath]; !ok {
		return false
	}

	var updated bool
	state.lock.Lock()
	defer state.lock.Unlock()
	if _, ok := state.commits[entryPath]; !ok {
		state.commits[entryPath] = commit
		updated = true
	}
	return updated
}

const getCommitsInfoPretty = "--pretty=format:%H %ct %s"

func getCommitsInfo(state *getCommitsInfoState, cache LastCommitCache) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	args := []string{"log", state.headCommit.ID.String(), getCommitsInfoPretty, "--name-status", "-c"}
	if len(state.treePath) > 0 {
		args = append(args, "--", state.treePath)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = state.headCommit.repo.Path

	readCloser, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	// it's okay to ignore the error returned by cmd.Wait(); we expect the
	// subprocess to sometimes have a non-zero exit status, since we may
	// prematurely close stdout, resulting in a broken pipe.
	defer cmd.Wait()

	numThreads := runtime.NumCPU()
	done := make(chan error, numThreads)
	for i := 0; i < numThreads; i++ {
		go targetedSearch(state, done, cache)
	}

	scanner := bufio.NewScanner(readCloser)
	err = state.processGitLogOutput(scanner)

	// it is important that we close stdout here; if we do not close
	// stdout, the subprocess will keep running, and the deffered call
	// cmd.Wait() may block for a long time.
	if closeErr := readCloser.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	for i := 0; i < numThreads; i++ {
		doneErr := <-done
		if doneErr != nil && err == nil {
			err = doneErr
		}
	}
	return err
}

func (state *getCommitsInfoState) processGitLogOutput(scanner *bufio.Scanner) error {
	// keep a local cache of seen paths to avoid acquiring a lock for paths
	// we've already seen
	seenPaths := make(map[string]struct{}, len(state.entryPaths))
	// number of consecutive commits without any finds
	coldStreak := 0
	var commit *Commit
	var err error
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 { // in-between commits
			numRemainingEntries := state.numRemainingEntries()
			if numRemainingEntries == 0 {
				break
			}
			if coldStreak >= deferToTargetedSearchColdStreak &&
				numRemainingEntries <= deferToTargetedSearchNumRemainingEntries {
				// stop this untargeted search, and let the targeted-search threads
				// finish the work
				break
			}
			continue
		}
		if line[0] >= 'A' && line[0] <= 'X' { // a file was changed by the current commit
			// look for the last tab, since for copies (C) and renames (R) two
			// filenames are printed: src, then dest
			tabIndex := strings.LastIndexByte(line, '\t')
			if tabIndex < 1 {
				return fmt.Errorf("misformatted line: %s", line)
			}
			entryPath, err := state.cleanEntryPath(line[tabIndex+1:])
			if err != nil {
				return err
			}
			if _, ok := seenPaths[entryPath]; !ok {
				if state.update(entryPath, commit) {
					coldStreak = 0
				}
				seenPaths[entryPath] = struct{}{}
			}
			continue
		}

		// a new commit
		commit, err = parseCommitInfo(line)
		if err != nil {
			return err
		}
		coldStreak++
	}
	return scanner.Err()
}

// parseCommitInfo parse a commit from a line of `git log` output. Expects the
// line to be formatted according to getCommitsInfoPretty.
func parseCommitInfo(line string) (*Commit, error) {
	if len(line) < 43 {
		return nil, fmt.Errorf("invalid git output: %s", line)
	}
	ref, err := NewIDFromString(line[:40])
	if err != nil {
		return nil, err
	}
	spaceIndex := strings.IndexByte(line[41:], ' ')
	if spaceIndex < 0 {
		return nil, fmt.Errorf("invalid git output: %s", line)
	}
	unixSeconds, err := strconv.Atoi(line[41 : 41+spaceIndex])
	if err != nil {
		return nil, err
	}
	message := line[spaceIndex+42:]
	return &Commit{
		ID:            ref,
		CommitMessage: message,
		Committer: &Signature{
			When: time.Unix(int64(unixSeconds), 0),
		},
	}, nil
}
