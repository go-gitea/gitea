// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"io"
)

// minLsTreeLineLen is a conservative lower bound on the length of one valid
// `git ls-tree` line: "100644 blob " plus a 40 character SHA-1 plus a tab and
// at least a one character name. SHA-256 object IDs are longer still.
const minLsTreeLineLen = 54

// ParseTreeEntries parses the output of a `git ls-tree -l` command.
func ParseTreeEntries(data []byte) ([]*TreeEntry, error) {
	return parseTreeEntries(data, nil)
}

// parseTreeEntries FIXME this function's design is not right, it should not make the caller read all data into memory
func parseTreeEntries(data []byte, ptree *Tree) ([]*TreeEntry, error) {
	// The capacity hint is only a hint to append, so bounding it cannot change the
	// result. It is bounded because the hint is derived from the input length: an
	// input made only of newlines would otherwise reserve one pointer per newline
	// before a single line is validated, and parseLsTreeLine below rejects the
	// very first one.
	//
	// The bound is proportional to the input rather than a constant, so a large
	// real tree still gets a useful hint: a valid ls-tree line carries a mode, a
	// type, a 40+ character object ID, a tab and a name, so it cannot be shorter
	// than minLsTreeLineLen bytes.
	nLines := bytes.Count(data, []byte{'\n'}) + 1
	if max := len(data)/minLsTreeLineLen + 1; nLines > max {
		nLines = max
	}
	entries := make([]*TreeEntry, 0, nLines)
	for pos := 0; pos < len(data); {
		posEnd := bytes.IndexByte(data[pos:], '\n')
		if posEnd == -1 {
			posEnd = len(data)
		} else {
			posEnd += pos
		}

		line := data[pos:posEnd]
		lsTreeLine, err := parseLsTreeLine(line)
		if err != nil {
			return nil, err
		}
		entry := &TreeEntry{
			ptree:     ptree,
			ID:        lsTreeLine.ID,
			entryMode: lsTreeLine.EntryMode,
			name:      lsTreeLine.Name,
			size:      lsTreeLine.Size.Value(),
			sized:     lsTreeLine.Size.Has(),
		}
		pos = posEnd + 1
		entries = append(entries, entry)
	}
	return entries, nil
}

var _ = catBatchParseTreeEntries // bypass "unused" lint because it is only used by "nogogit"

func catBatchParseTreeEntries(objectFormat ObjectFormat, ptree *Tree, rd BufferedReader, sz int64) ([]*TreeEntry, error) {
	entries := make([]*TreeEntry, 0, 10)

loop:
	for sz > 0 {
		mode, fname, objID, count, err := ParseCatFileTreeLine(objectFormat, rd)
		if err != nil {
			if err == io.EOF {
				break loop
			}
			return nil, err
		}
		sz -= int64(count)
		entry := new(TreeEntry)
		entry.ptree = ptree
		entry.entryMode = mode
		entry.ID = objID
		entry.name = fname
		entries = append(entries, entry)
	}
	if _, err := rd.Discard(1); err != nil {
		return entries, err
	}

	return entries, nil
}
