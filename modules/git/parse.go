// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/optional"
)

var sepSpace = []byte{' '}

type LsTreeEntry struct {
	ID        ObjectID
	EntryMode EntryMode
	Name      string
	Size      optional.Option[int64]
}

func parseLsTreeLine(line []byte) (*LsTreeEntry, error) {
	// expect line to be of the form:
	// <mode> <type> <sha> <space-padded-size>\t<filename>
	// <mode> <type> <sha>\t<filename>

	var err error
	posTab := bytes.IndexByte(line, '\t')
	if posTab == -1 {
		return nil, fmt.Errorf("invalid ls-tree output (no tab): %q", line)
	}

	entry := new(LsTreeEntry)

	entryAttrs := line[:posTab]
	entryName := line[posTab+1:]

	entryMode, entryAttrs, _ := bytes.Cut(entryAttrs, sepSpace)
	_ /* entryType */, entryAttrs, _ = bytes.Cut(entryAttrs, sepSpace) // the type is not used, the mode is enough to determine the type
	entryObjectID, entryAttrs, _ := bytes.Cut(entryAttrs, sepSpace)
	if len(entryAttrs) > 0 {
		entrySize := entryAttrs // the last field is the space-padded-size
		size, _ := strconv.ParseInt(strings.TrimSpace(string(entrySize)), 10, 64)
		entry.Size = optional.Some(size)
	}

	entry.EntryMode, err = ParseEntryMode(string(entryMode))
	if err != nil || entry.EntryMode == EntryModeNoEntry {
		return nil, fmt.Errorf("invalid ls-tree output (invalid mode): %q, err: %w", line, err)
	}

	entry.ID, err = NewIDFromString(string(entryObjectID))
	if err != nil {
		return nil, fmt.Errorf("invalid ls-tree output (invalid object id): %q, err: %w", line, err)
	}

	if len(entryName) > 0 && entryName[0] == '"' {
		entry.Name, err = strconv.Unquote(string(entryName))
		if err != nil {
			return nil, fmt.Errorf("invalid ls-tree output (invalid name): %q, err: %w", line, err)
		}
	} else {
		entry.Name = string(entryName)
	}
	return entry, nil
}
