// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// ParseTreeEntries parses the output of a `git ls-tree -l` command.
func ParseTreeEntries(data []byte) ([]*TreeEntry, error) {
	return parseTreeEntries(data, nil)
}

var sepSpace = []byte{' '}

func parseTreeEntries(data []byte, ptree *Tree) ([]*TreeEntry, error) {
	var err error
	entries := make([]*TreeEntry, 0, bytes.Count(data, []byte{'\n'})+1)
	for pos := 0; pos < len(data); {
		// expect line to be of the form:
		// <mode> <type> <sha> <space-padded-size>\t<filename>
		// <mode> <type> <sha>\t<filename>
		posEnd := bytes.IndexByte(data[pos:], '\n')
		if posEnd == -1 {
			posEnd = len(data)
		} else {
			posEnd += pos
		}
		line := data[pos:posEnd]
		posTab := bytes.IndexByte(line, '\t')
		if posTab == -1 {
			return nil, fmt.Errorf("invalid ls-tree output (no tab): %q", line)
		}

		entry := new(TreeEntry)
		entry.ptree = ptree

		entryAttrs := line[:posTab]
		entryName := line[posTab+1:]

		entryMode, entryAttrs, _ := bytes.Cut(entryAttrs, sepSpace)
		_ /* entryType */, entryAttrs, _ = bytes.Cut(entryAttrs, sepSpace) // the type is not used, the mode is enough to determine the type
		entryObjectID, entryAttrs, _ := bytes.Cut(entryAttrs, sepSpace)
		if len(entryAttrs) > 0 {
			entrySize := entryAttrs // the last field is the space-padded-size
			entry.size, _ = strconv.ParseInt(strings.TrimSpace(string(entrySize)), 10, 64)
			entry.sized = true
		}

		switch string(entryMode) {
		case "100644":
			entry.entryMode = EntryModeBlob
		case "100755":
			entry.entryMode = EntryModeExec
		case "120000":
			entry.entryMode = EntryModeSymlink
		case "160000":
			entry.entryMode = EntryModeCommit
		case "040000", "040755": // git uses 040000 for tree object, but some users may get 040755 for unknown reasons
			entry.entryMode = EntryModeTree
		default:
			return nil, fmt.Errorf("unknown type: %v", string(entryMode))
		}

		entry.ID, err = NewIDFromString(string(entryObjectID))
		if err != nil {
			return nil, fmt.Errorf("invalid ls-tree output (invalid object id): %q, err: %w", line, err)
		}

		if len(entryName) > 0 && entryName[0] == '"' {
			entry.name, err = strconv.Unquote(string(entryName))
			if err != nil {
				return nil, fmt.Errorf("invalid ls-tree output (invalid name): %q, err: %w", line, err)
			}
		} else {
			entry.name = string(entryName)
		}

		pos = posEnd + 1
		entries = append(entries, entry)
	}
	return entries, nil
}

func catBatchParseTreeEntries(objectFormat ObjectFormat, ptree *Tree, rd *bufio.Reader, sz int64) ([]*TreeEntry, error) {
	fnameBuf := make([]byte, 4096)
	modeBuf := make([]byte, 40)
	shaBuf := make([]byte, objectFormat.FullLength())
	entries := make([]*TreeEntry, 0, 10)

loop:
	for sz > 0 {
		mode, fname, sha, count, err := ParseTreeLine(objectFormat, rd, modeBuf, fnameBuf, shaBuf)
		if err != nil {
			if err == io.EOF {
				break loop
			}
			return nil, err
		}
		sz -= int64(count)
		entry := new(TreeEntry)
		entry.ptree = ptree

		switch string(mode) {
		case "100644":
			entry.entryMode = EntryModeBlob
		case "100755":
			entry.entryMode = EntryModeExec
		case "120000":
			entry.entryMode = EntryModeSymlink
		case "160000":
			entry.entryMode = EntryModeCommit
		case "40000", "40755": // git uses 40000 for tree object, but some users may get 40755 for unknown reasons
			entry.entryMode = EntryModeTree
		default:
			log.Debug("Unknown mode: %v", string(mode))
			return nil, fmt.Errorf("unknown mode: %v", string(mode))
		}

		entry.ID = objectFormat.MustID(sha)
		entry.name = string(fname)
		entries = append(entries, entry)
	}
	if _, err := rd.Discard(1); err != nil {
		return entries, err
	}

	return entries, nil
}
