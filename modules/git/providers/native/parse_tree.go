// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bytes"
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/git/service"
)

// ParseTreeEntries parses the output of a `git ls-tree` command.
func ParseTreeEntries(data []byte) ([]service.TreeEntry, error) {
	return parseTreeEntries(data, nil)
}

func parseTreeEntries(data []byte, ptree service.Tree) ([]service.TreeEntry, error) {
	entries := make([]service.TreeEntry, 0, 10)
	for pos := 0; pos < len(data); {
		// expect line to be of the form "<mode> <type> <sha>\t<filename>"
		entry := new(TreeEntry)
		entry.ptree = ptree
		if pos+6 > len(data) {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}
		switch string(data[pos : pos+6]) {
		case "100644":
			entry.entryMode = service.EntryModeBlob
			pos += 12 // skip over "100644 blob "
		case "100755":
			entry.entryMode = service.EntryModeExec
			pos += 12 // skip over "100755 blob "
		case "120000":
			entry.entryMode = service.EntryModeSymlink
			pos += 12 // skip over "120000 blob "
		case "160000":
			entry.entryMode = service.EntryModeCommit
			pos += 14 // skip over "160000 object "
		case "040000":
			entry.entryMode = service.EntryModeTree
			pos += 12 // skip over "040000 tree "
		default:
			return nil, fmt.Errorf("unknown type: %v", string(data[pos:pos+6]))
		}

		if pos+40 > len(data) {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}
		entry.hash = StringHash(string(data[pos : pos+40]))
		if ptree != nil {
			entry.repo = ptree.Repository()
		}

		pos += 41 // skip over sha and trailing space

		end := pos + bytes.IndexByte(data[pos:], '\n')
		if end < pos {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}

		// In case entry name is surrounded by double quotes(it happens only in git-shell).
		if data[pos] == '"' {
			var err error
			entry.name, err = strconv.Unquote(string(data[pos:end]))
			if err != nil {
				return nil, fmt.Errorf("Invalid ls-tree output: %v", err)
			}
		} else {
			entry.name = string(data[pos:end])
		}

		pos = end + 1
		entries = append(entries, entry)
	}
	return entries, nil
}
