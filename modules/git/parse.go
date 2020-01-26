// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
	"strconv"

	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// ParseTreeEntries parses the output of a `git ls-tree` command.
func ParseTreeEntries(data []byte) ([]*TreeEntry, error) {
	return parseTreeEntries(data, nil)
}

func parseTreeEntries(data []byte, ptree *Tree) ([]*TreeEntry, error) {
	entries := make([]*TreeEntry, 0, 10)
	for pos := 0; pos < len(data); {
		// expect line to be of the form "<mode> <type> <sha>\t<filename>"
		entry := new(TreeEntry)
		entry.gogitTreeEntry = &object.TreeEntry{}
		entry.ptree = ptree
		if pos+6 > len(data) {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}
		switch string(data[pos : pos+6]) {
		case "100644":
			entry.gogitTreeEntry.Mode = filemode.Regular
			pos += 12 // skip over "100644 blob "
		case "100755":
			entry.gogitTreeEntry.Mode = filemode.Executable
			pos += 12 // skip over "100755 blob "
		case "120000":
			entry.gogitTreeEntry.Mode = filemode.Symlink
			pos += 12 // skip over "120000 blob "
		case "160000":
			entry.gogitTreeEntry.Mode = filemode.Submodule
			pos += 14 // skip over "160000 object "
		case "040000":
			entry.gogitTreeEntry.Mode = filemode.Dir
			pos += 12 // skip over "040000 tree "
		default:
			return nil, fmt.Errorf("unknown type: %v", string(data[pos:pos+6]))
		}

		if pos+40 > len(data) {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}
		id, err := NewIDFromString(string(data[pos : pos+40]))
		if err != nil {
			return nil, fmt.Errorf("Invalid ls-tree output: %v", err)
		}
		entry.ID = id
		entry.gogitTreeEntry.Hash = id
		pos += 41 // skip over sha and trailing space

		end := pos + bytes.IndexByte(data[pos:], '\n')
		if end < pos {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}

		// In case entry name is surrounded by double quotes(it happens only in git-shell).
		if data[pos] == '"' {
			entry.gogitTreeEntry.Name, err = strconv.Unquote(string(data[pos:end]))
			if err != nil {
				return nil, fmt.Errorf("Invalid ls-tree output: %v", err)
			}
		} else {
			entry.gogitTreeEntry.Name = string(data[pos:end])
		}

		pos = end + 1
		entries = append(entries, entry)
	}
	return entries, nil
}
