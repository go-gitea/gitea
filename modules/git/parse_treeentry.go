// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"fmt"
	"io"

	"code.gitea.io/gitea/modules/log"
)

// ParseTreeEntries parses the output of a `git ls-tree -l` command.
// FIXME this function's design is not right, it should not make the caller read all data into memory
func ParseTreeEntries(data []byte) ([]*TreeEntry, error) {
	entries := make([]*TreeEntry, 0, bytes.Count(data, []byte{'\n'})+1)
	for pos := 0; pos < len(data); {
		posEnd := bytes.IndexByte(data[pos:], '\n')
		if posEnd == -1 {
			posEnd = len(data)
		} else {
			posEnd += pos
		}

		line := data[pos:posEnd]
		entry, err := parseTreeEntry(line)
		if err != nil {
			return nil, err
		}
		pos = posEnd + 1
		entries = append(entries, entry)
	}
	return entries, nil
}

func catBatchParseTreeEntries(objectFormat ObjectFormat, rd BufferedReader, sz int64) ([]*TreeEntry, error) {
	fnameBuf := make([]byte, 4096)
	modeBuf := make([]byte, 40)
	shaBuf := make([]byte, objectFormat.FullLength())
	entries := make([]*TreeEntry, 0, 10)

loop:
	for sz > 0 {
		mode, fname, sha, count, err := ParseCatFileTreeLine(objectFormat, rd, modeBuf, fnameBuf, shaBuf)
		if err != nil {
			if err == io.EOF {
				break loop
			}
			return nil, err
		}
		sz -= int64(count)
		entry := new(TreeEntry)

		switch string(mode) {
		case "100644":
			entry.EntryMode = EntryModeBlob
		case "100755":
			entry.EntryMode = EntryModeExec
		case "120000":
			entry.EntryMode = EntryModeSymlink
		case "160000":
			entry.EntryMode = EntryModeCommit
		case "40000", "40755": // git uses 40000 for tree object, but some users may get 40755 for unknown reasons
			entry.EntryMode = EntryModeTree
		default:
			log.Debug("Unknown mode: %v", string(mode))
			return nil, fmt.Errorf("unknown mode: %v", string(mode))
		}

		entry.ID = objectFormat.MustID(sha)
		entry.Name = string(fname)
		entries = append(entries, entry)
	}
	if _, err := rd.Discard(1); err != nil {
		return entries, err
	}

	return entries, nil
}
