// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build nogogit

package git

import (
	"bufio"
	"bytes"
	"io"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
)

// CommitInfo describes the first commit with the provided entry
type CommitInfo struct {
	Entry         *TreeEntry
	Commit        *Commit
	SubModuleFile *SubModuleFile
}

// ReadBatchLine reads the header line from cat-file --batch
// We expect:
// <sha> SP <type> SP <size> LF
func ReadBatchLine(rd *bufio.Reader) (sha []byte, typ string, size int64, err error) {
	sha, err = rd.ReadBytes(' ')
	if err != nil {
		return
	}
	sha = sha[:len(sha)-1]

	typ, err = rd.ReadString(' ')
	if err != nil {
		return
	}
	typ = typ[:len(typ)-1]

	var sizeStr string
	sizeStr, err = rd.ReadString('\n')
	if err != nil {
		return
	}

	size, err = strconv.ParseInt(sizeStr[:len(sizeStr)-1], 10, 64)
	return
}

func GetTagObjectID(rd *bufio.Reader, size int64) (string, error) {
	id := ""
	var n int64
headerLoop:
	for {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			return "", err
		}
		n += int64(len(line))
		idx := bytes.Index(line, []byte{' '})
		if idx < 0 {
			continue
		}

		if string(line[:idx]) == "object" {
			id = string(line[idx+1 : len(line)-1])
			break headerLoop
		}
	}

	// Discard the rest of the tag
	discard := size - n
	for discard > math.MaxInt32 {
		_, err := rd.Discard(math.MaxInt32)
		if err != nil {
			return id, err
		}
		discard -= math.MaxInt32
	}
	_, err := rd.Discard(int(discard))
	return id, err
}

func ParseTreeLine(rd *bufio.Reader) (mode, fname, sha string, n int, err error) {
	mode, err = rd.ReadString(' ')
	if err != nil {
		return
	}
	n += len(mode)
	mode = mode[:len(mode)-1]

	fname, err = rd.ReadString('\x00')
	if err != nil {
		return
	}
	n += len(fname)
	fname = fname[:len(fname)-1]

	shaBytes := make([]byte, 20)
	read, err := rd.Read(shaBytes)
	if err != nil {
		return
	}
	n += read
	sha = MustID(shaBytes).String()
	return
}

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(commit *Commit, treePath string, cache *LastCommitCache) ([]CommitInfo, *Commit, error) {
	entryPaths := make([]string, len(tes)+1)
	// Get the commit for the treePath itself
	entryPaths[0] = ""
	for i, entry := range tes {
		entryPaths[i+1] = entry.Name()
	}

	var err error

	var revs map[string]*Commit
	if cache != nil {
		var unHitPaths []string
		revs, unHitPaths, err = getLastCommitForPathsByCache(commit.ID.String(), treePath, entryPaths, cache)
		if err != nil {
			return nil, nil, err
		}
		if len(unHitPaths) > 0 {
			sort.Strings(unHitPaths)
			commits, err := GetLastCommitForPaths(commit, treePath, unHitPaths)
			if err != nil {
				return nil, nil, err
			}

			for i, found := range commits {
				if err := cache.Put(commit.ID.String(), path.Join(treePath, unHitPaths[i]), found.ID.String()); err != nil {
					return nil, nil, err
				}
				revs[unHitPaths[i]] = found
			}
		}
	} else {
		sort.Strings(entryPaths)
		revs = map[string]*Commit{}
		var foundCommits []*Commit
		foundCommits, err = GetLastCommitForPaths(commit, treePath, entryPaths)
		for i, found := range foundCommits {
			revs[entryPaths[i]] = found
		}
	}
	if err != nil {
		return nil, nil, err
	}

	commitsInfo := make([]CommitInfo, len(tes))
	for i, entry := range tes {
		commitsInfo[i] = CommitInfo{
			Entry: entry,
		}
		if entryCommit, ok := revs[entry.Name()]; ok {
			commitsInfo[i].Commit = entryCommit
			if entry.IsSubModule() {
				subModuleURL := ""
				var fullPath string
				if len(treePath) > 0 {
					fullPath = treePath + "/" + entry.Name()
				} else {
					fullPath = entry.Name()
				}
				if subModule, err := commit.GetSubModule(fullPath); err != nil {
					return nil, nil, err
				} else if subModule != nil {
					subModuleURL = subModule.URL
				}
				subModuleFile := NewSubModuleFile(entryCommit, subModuleURL, entry.ID.String())
				commitsInfo[i].SubModuleFile = subModuleFile
			}
		}
	}

	// Retrieve the commit for the treePath itself (see above). We basically
	// get it for free during the tree traversal and it's used for listing
	// pages to display information about newest commit for a given path.
	var treeCommit *Commit
	var ok bool
	if treePath == "" {
		treeCommit = commit
	} else if treeCommit, ok = revs[""]; ok {
		treeCommit.repo = commit.repo
	}
	return commitsInfo, treeCommit, nil
}

func getLastCommitForPathsByCache(commitID, treePath string, paths []string, cache *LastCommitCache) (map[string]*Commit, []string, error) {
	var unHitEntryPaths []string
	var results = make(map[string]*Commit)
	for _, p := range paths {
		lastCommit, err := cache.Get(commitID, path.Join(treePath, p))
		if err != nil {
			return nil, nil, err
		}
		if lastCommit != nil {
			results[p] = lastCommit.(*Commit)
			continue
		}

		unHitEntryPaths = append(unHitEntryPaths, p)
	}

	return results, unHitEntryPaths, nil
}

// GetLastCommitForPaths returns last commit information
func GetLastCommitForPaths(commit *Commit, treePath string, paths []string) ([]*Commit, error) {
	// We read backwards from the commit to obtain all of the commits
	path2idx := make(map[string]int, len(paths))
	for i, path := range paths {
		path2idx[path] = i
	}

	// We'll do this by using rev-list to provide us with parent commits in order
	revListReader, revListWriter := io.Pipe()
	defer func() {
		_ = revListWriter.Close()
		_ = revListReader.Close()
	}()

	go func() {
		stderr := strings.Builder{}
		err := NewCommand("rev-list", commit.ID.String()).RunInDirPipeline(commit.repo.Path, revListWriter, &stderr)
		if err != nil {
			_ = revListWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = revListWriter.Close()
		}
	}()

	// We feed the commits in order into cat-file --batch, followed by their trees and sub trees as necessary.
	// so let's create a batch stdin and stdout
	batchStdinReader, batchStdinWriter := io.Pipe()
	batchStdoutReader, batchStdoutWriter := io.Pipe()
	defer func() {
		_ = batchStdinReader.Close()
		_ = batchStdinWriter.Close()
		_ = batchStdoutReader.Close()
		_ = batchStdoutWriter.Close()
	}()

	go func() {
		stderr := strings.Builder{}
		err := NewCommand("cat-file", "--batch").RunInDirFullPipeline(commit.repo.Path, batchStdoutWriter, &stderr, batchStdinReader)
		if err != nil {
			_ = revListWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = revListWriter.Close()
		}
	}()

	// For simplicities sake we'll us a buffered reader
	batchReader := bufio.NewReader(batchStdoutReader)

	// commits is the returnable commits matching the paths provided
	commits := make([]*Commit, len(paths))
	// ids are the blob/tree ids for the paths
	ids := make([]string, len(paths))
	// found is a shortcut to help break out of parsing early
	found := make([]bool, len(paths))

	// We'll use a scanner for the revList because it's simpler than a bufio.Reader
	scan := bufio.NewScanner(revListReader)
revListLoop:
	for scan.Scan() {
		// Get the next parent commit ID
		commitID := scan.Bytes()

		// push the commit to the cat-file --batch process
		_, err := batchStdinWriter.Write(commitID)
		if err != nil {
			return nil, err
		}
		_, err = batchStdinWriter.Write([]byte{'\n'})
		if err != nil {
			return nil, err
		}

		var curCommit *Commit

		currentPath := ""

		i := 0
		commitDone := false
	commitReadingLoop:
		for !commitDone {
			_, typ, size, err := ReadBatchLine(batchReader)
			if err != nil {
				return nil, err
			}

			switch typ {
			case "tag":
				// This shouldn't happen but if it does well just get the commit and try again
				id, err := GetTagObjectID(batchReader, size)
				if err != nil {
					return nil, err
				}
				_, err = batchStdinWriter.Write([]byte(id + "\n"))
				if err != nil {
					return nil, err
				}
				continue
			case "commit":
				// Read in the commit to get its tree and in case this is one of the last used commits
				curCommit, err = CommitFromReader(commit.repo, MustIDFromString(string(commitID)), io.LimitReader(batchReader, int64(size)))
				if err != nil {
					return nil, err
				}

				// Get the tree for the commit
				id := curCommit.Tree.ID.String()
				// OK if the target tree path is "" and the "" is in the paths just set this now
				if treePath == "" && paths[0] == "" {
					if i == 0 {
						i++
					}
					// If this is the first time we see this set the id appropriate for this paths to this tree and set the last commit to curCommit
					if ids[0] == "" {
						log("setting initial id to: %s on commit %s", string(id), curCommit.ID.String())
						ids[0] = id
						commits[0] = curCommit
					} else if ids[0] != id {
						// Else if the last id doesn't match this we've found the last commit that added this
						found[0] = true

						// check if that completes our list
						done := true
						for _, find := range found {
							if !find {
								done = false
								break
							}
						}
						if done {
							break revListLoop
						}
					} else if !found[0] {
						// Finally if we haven't found the commit set the curCommit to this
						commits[0] = curCommit
					}
				}

				// in the unlikely event that we've now done all the paths
				if i >= len(paths) {
					commitDone = true
					continue
				}

				// Finally add the tree back in to batch reader
				_, err = batchStdinWriter.Write([]byte(id + "\n"))
				if err != nil {
					return nil, err
				}
				continue
			case "tree":
				// Handle trees

				// n is counter for file position in the tree file
				var n int64

				// Two options: currentPath is the targetTreepath
				if treePath == currentPath {
					// We are in the right directory
					// Parse each tree line in turn. (don't care about mode here.)
					for n < size {
						_, fname, sha, count, err := ParseTreeLine(batchReader)
						if err != nil {
							return nil, err
						}
						n += int64(count)

						idx, ok := path2idx[fname]
						if ok {
							// Now if this is the first time round set the initial Blob(ish) SHA ID and the commit
							if ids[idx] == "" {
								ids[idx] = sha
								commits[idx] = curCommit
							} else if !found[idx] {
								// if we've not already found this path's commit
								if ids[idx] != sha {
									// if the SHA is different we've now found the commit for this path
									found[idx] = true

									// check if we've found all the paths
									done := true
									for _, find := range found {
										if !find {
											done = false
											break
										}
									}
									if done {
										break revListLoop
									}
								} else {
									commits[idx] = curCommit
								}
							}
						}

						// if fname > paths[i] well paths[i] must not be present in this set it to found
						if n >= size {
							done := true
							for i := range paths {
								if commits[i] != curCommit {
									found[i] = true
								} else {
									done = done && found[i]
								}
							}

							// check if we're really done
							if done {
								break revListLoop
							}
							break commitReadingLoop
						}
					}
					// We should not be able to get here...
					break revListLoop
				}

				// We're in the wrong directory
				// Find target directory in this directory
				idx := len(currentPath)
				if idx > 0 {
					idx++
				}
				target := strings.SplitN(treePath[idx:], "/", 2)[0]

				treeID := ""
				for n < size {
					// Read each tree entry in turn
					mode, fname, sha, count, err := ParseTreeLine(batchReader)
					if err != nil {
						return nil, err
					}
					n += int64(count)

					// if we have found the target directory
					if fname == target && ToEntryMode(mode) == EntryModeTree {
						treeID = sha
						break
					} else if fname > target {
						break
					}
				}
				// if we haven't found a treeID for the target directory our search is over
				if treeID == "" {
					break revListLoop
				}

				if n < size {
					// Discard any remaining entries in the current tree
					discard := size - n
					for discard > math.MaxInt32 {
						_, err := batchReader.Discard(math.MaxInt32)
						if err != nil {
							return nil, err
						}
						discard -= math.MaxInt32
					}
					_, err := batchReader.Discard(int(discard))
					if err != nil {
						return nil, err
					}
				}

				// add the target to the current path
				if idx > 0 {
					currentPath += "/"
				}
				currentPath += target

				// if we've now found the curent path check its sha id and commit status
				if treePath == currentPath && paths[0] == "" {
					if i == 0 {
						i++
					}
					if ids[0] == "" {
						ids[0] = treeID
						commits[0] = curCommit
					} else if !found[0] {
						if ids[0] != treeID {
							found[0] = true

							// check if that completes our list
							done := true
							for _, find := range found {
								if !find {
									done = false
									break
								}
							}
							if done {
								break revListLoop
							}
						} else if !found[0] {
							commits[0] = curCommit
						}
					}
				}
				_, err = batchStdinWriter.Write([]byte(treeID + "\n"))
				if err != nil {
					return nil, err
				}
				continue
			}
		}
	}

	return commits, scan.Err()
}
