// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/hash"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ParseTreeEntries parses the output of a `git ls-tree -l` command.
func ParseTreeEntries(data []byte) ([]*TreeEntry, error) {
	return parseTreeEntries(data, nil)
}

func parseTreeEntries(data []byte, ptree *Tree) ([]*TreeEntry, error) {
	entries := make([]*TreeEntry, 0, 10)
	for pos := 0; pos < len(data); {
		// expect line to be of the form "<mode> <type> <sha> <space-padded-size>\t<filename>"
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

		// in hex format, not byte format ....
		if pos+hash.Size*2 > len(data) {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}
		var err error
		entry.ID, err = NewIDFromString(string(data[pos : pos+hash.Size*2]))
		if err != nil {
			return nil, fmt.Errorf("invalid ls-tree output: %w", err)
		}
		entry.gogitTreeEntry.Hash = plumbing.Hash(entry.ID.RawValue())
		pos += 41 // skip over sha and trailing space

		end := pos + bytes.IndexByte(data[pos:], '\t')
		if end < pos {
			return nil, fmt.Errorf("Invalid ls-tree -l output: %s", string(data))
		}
		entry.size, _ = strconv.ParseInt(strings.TrimSpace(string(data[pos:end])), 10, 64)
		entry.sized = true

		pos = end + 1

		end = pos + bytes.IndexByte(data[pos:], '\n')
		if end < pos {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}

		// In case entry name is surrounded by double quotes(it happens only in git-shell).
		if data[pos] == '"' {
			var err error
			entry.gogitTreeEntry.Name, err = strconv.Unquote(string(data[pos:end]))
			if err != nil {
				return nil, fmt.Errorf("Invalid ls-tree output: %w", err)
			}
		} else {
			entry.gogitTreeEntry.Name = string(data[pos:end])
		}

		pos = end + 1
		entries = append(entries, entry)
	}
	return entries, nil
}
