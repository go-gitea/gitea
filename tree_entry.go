// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// EntryMode the type of the object in the git tree
type EntryMode int

// There are only a few file modes in Git. They look like unix file modes, but they can only be
// one of these.
const (
	// EntryModeBlob
	EntryModeBlob EntryMode = 0100644
	// EntryModeExec
	EntryModeExec EntryMode = 0100755
	// EntryModeSymlink
	EntryModeSymlink EntryMode = 0120000
	// EntryModeCommit
	EntryModeCommit EntryMode = 0160000
	// EntryModeTree
	EntryModeTree EntryMode = 0040000
)

// TreeEntry the leaf in the git tree
type TreeEntry struct {
	ID   SHA1
	Type ObjectType

	mode EntryMode
	name string

	ptree *Tree

	commited bool

	size  int64
	sized bool
}

// Name returns the name of the entry
func (te *TreeEntry) Name() string {
	return te.name
}

// Size returns the size of the entry
func (te *TreeEntry) Size() int64 {
	if te.IsDir() {
		return 0
	} else if te.sized {
		return te.size
	}

	stdout, err := NewCommand("cat-file", "-s", te.ID.String()).RunInDir(te.ptree.repo.Path)
	if err != nil {
		return 0
	}

	te.sized = true
	te.size, _ = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
	return te.size
}

// IsSubModule if the entry is a sub module
func (te *TreeEntry) IsSubModule() bool {
	return te.mode == EntryModeCommit
}

// IsDir if the entry is a sub dir
func (te *TreeEntry) IsDir() bool {
	return te.mode == EntryModeTree
}

// IsLink if the entry is a symlink
func (te *TreeEntry) IsLink() bool {
	return te.mode == EntryModeSymlink
}

// Blob retrun the blob object the entry
func (te *TreeEntry) Blob() *Blob {
	return &Blob{
		repo:      te.ptree.repo,
		TreeEntry: te,
	}
}

// GetSubJumpablePathName return the full path of subdirectory jumpable ( contains only one directory )
func (te *TreeEntry) GetSubJumpablePathName() string {
	if te.IsSubModule() || !te.IsDir() {
		return ""
	}
	tree, err := te.ptree.SubTree(te.name)
	if err != nil {
		return te.name
	}
	entries, _ := tree.ListEntries()
	if len(entries) == 1 && entries[0].IsDir() {
		name := entries[0].GetSubJumpablePathName()
		if name != "" {
			return te.name + "/" + name
		}
	}
	return te.name
}

// Entries a list of entry
type Entries []*TreeEntry

var sorter = []func(t1, t2 *TreeEntry) bool{
	func(t1, t2 *TreeEntry) bool {
		return (t1.IsDir() || t1.IsSubModule()) && !t2.IsDir() && !t2.IsSubModule()
	},
	func(t1, t2 *TreeEntry) bool {
		return t1.name < t2.name
	},
}

func (tes Entries) Len() int      { return len(tes) }
func (tes Entries) Swap(i, j int) { tes[i], tes[j] = tes[j], tes[i] }
func (tes Entries) Less(i, j int) bool {
	t1, t2 := tes[i], tes[j]
	var k int
	for k = 0; k < len(sorter)-1; k++ {
		s := sorter[k]
		switch {
		case s(t1, t2):
			return true
		case s(t2, t1):
			return false
		}
	}
	return sorter[k](t1, t2)
}

// Sort sort the list of entry
func (tes Entries) Sort() {
	sort.Sort(tes)
}

// getCommitInfoState transient state for getting commit info for entries
type getCommitInfoState struct {
	entries        map[string]*TreeEntry // map from filepath to entry
	commits        map[string]*Commit    // map from filepath to commit
	lastCommitHash string
	lastCommit     *Commit
	treePath       string
	headCommit     *Commit
	nextSearchSize int // next number of commits to search for
}

func initGetCommitInfoState(entries Entries, headCommit *Commit, treePath string) *getCommitInfoState {
	entriesByPath := make(map[string]*TreeEntry, len(entries))
	for _, entry := range entries {
		entriesByPath[path.Join(treePath, entry.Name())] = entry
	}
	if treePath = path.Clean(treePath); treePath == "." {
		treePath = ""
	}
	return &getCommitInfoState{
		entries:        entriesByPath,
		commits:        make(map[string]*Commit, len(entriesByPath)),
		treePath:       treePath,
		headCommit:     headCommit,
		nextSearchSize: 16,
	}
}

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(commit *Commit, treePath string) ([][]interface{}, error) {
	state := initGetCommitInfoState(tes, commit, treePath)
	if err := getCommitsInfo(state); err != nil {
		return nil, err
	}

	commitsInfo := make([][]interface{}, len(tes))
	for i, entry := range tes {
		commit = state.commits[path.Join(treePath, entry.Name())]
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

func (state *getCommitInfoState) nextCommit(hash string) {
	state.lastCommitHash = hash
	state.lastCommit = nil
}

func (state *getCommitInfoState) commit() (*Commit, error) {
	var err error
	if state.lastCommit == nil {
		state.lastCommit, err = state.headCommit.repo.GetCommit(state.lastCommitHash)
	}
	return state.lastCommit, err
}

func (state *getCommitInfoState) update(entryPath string) error {
	var entryNameStartIndex int
	if len(state.treePath) > 0 {
		entryNameStartIndex = len(state.treePath) + 1
	}

	if index := strings.IndexByte(entryPath[entryNameStartIndex:], '/'); index >= 0 {
		entryPath = entryPath[:entryNameStartIndex+index]
	}

	if _, ok := state.entries[entryPath]; !ok {
		return nil
	} else if _, ok := state.commits[entryPath]; ok {
		return nil
	}

	var err error
	state.commits[entryPath], err = state.commit()
	return err
}

func getCommitsInfo(state *getCommitInfoState) error {
	for len(state.entries) > len(state.commits) {
		if err := getNextCommitInfos(state); err != nil {
			return err
		}
	}
	return nil
}

func getNextCommitInfos(state *getCommitInfoState) error {
	logOutput, err := logCommand(state.lastCommitHash, state).RunInDir(state.headCommit.repo.Path)
	if err != nil {
		return err
	}
	lines := strings.Split(logOutput, "\n")
	i := 0
	for i < len(lines) {
		state.nextCommit(lines[i])
		i++
		for ; i < len(lines); i++ {
			entryPath := lines[i]
			if entryPath == "" {
				break
			}
			if entryPath[0] == '"' {
				entryPath, err = strconv.Unquote(entryPath)
				if err != nil {
					return fmt.Errorf("Unquote: %v", err)
				}
			}
			state.update(entryPath)
		}
		i++ // skip blank line
		if len(state.entries) == len(state.commits) {
			break
		}
	}
	return nil
}

func logCommand(exclusiveStartHash string, state *getCommitInfoState) *Command {
	var commitHash string
	if len(exclusiveStartHash) == 0 {
		commitHash = state.headCommit.ID.String()
	} else {
		commitHash = exclusiveStartHash + "^"
	}
	var command *Command
	numRemainingEntries := len(state.entries) - len(state.commits)
	if numRemainingEntries < 32 {
		searchSize := (numRemainingEntries + 1) / 2
		command = NewCommand("log", prettyLogFormat, "--name-only",
			"-"+strconv.Itoa(searchSize), commitHash, "--")
		for entryPath := range state.entries {
			if _, ok := state.commits[entryPath]; !ok {
				command.AddArguments(entryPath)
			}
		}
	} else {
		command = NewCommand("log", prettyLogFormat, "--name-only",
			"-"+strconv.Itoa(state.nextSearchSize), commitHash, "--", state.treePath)
	}
	state.nextSearchSize += state.nextSearchSize
	return command
}
