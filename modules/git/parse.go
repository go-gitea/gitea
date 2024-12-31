// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

var sepSpace = []byte{' '}

func parseLsTreeLine(line []byte) (*TreeEntry, error) {
	// expect line to be of the form:
	// <mode> <type> <sha> <space-padded-size>\t<filename>
	// <mode> <type> <sha>\t<filename>

	var err error
	posTab := bytes.IndexByte(line, '\t')
	if posTab == -1 {
		return nil, fmt.Errorf("invalid ls-tree output (no tab): %q", line)
	}

	entry := new(TreeEntry)

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
	return entry, nil
}
