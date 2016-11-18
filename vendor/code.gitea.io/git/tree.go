// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
	"strings"
)

// Tree represents a flat directory listing.
type Tree struct {
	ID   sha1
	repo *Repository

	// parent tree
	ptree *Tree

	entries       Entries
	entriesParsed bool
}

func NewTree(repo *Repository, id sha1) *Tree {
	return &Tree{
		ID:   id,
		repo: repo,
	}
}

var escapeChar = []byte("\\")

// UnescapeChars reverses escaped characters.
func UnescapeChars(in []byte) []byte {
	if bytes.Index(in, escapeChar) == -1 {
		return in
	}

	endIdx := len(in) - 1
	isEscape := false
	out := make([]byte, 0, endIdx+1)
	for i := range in {
		if in[i] == '\\' && !isEscape {
			isEscape = true
			continue
		}
		isEscape = false
		out = append(out, in[i])
	}
	return out
}

// parseTreeData parses tree information from the (uncompressed) raw
// data from the tree object.
func parseTreeData(tree *Tree, data []byte) ([]*TreeEntry, error) {
	entries := make([]*TreeEntry, 0, 10)
	l := len(data)
	pos := 0
	for pos < l {
		entry := new(TreeEntry)
		entry.ptree = tree
		step := 6
		switch string(data[pos : pos+step]) {
		case "100644":
			entry.mode = ENTRY_MODE_BLOB
			entry.Type = OBJECT_BLOB
		case "100755":
			entry.mode = ENTRY_MODE_EXEC
			entry.Type = OBJECT_BLOB
		case "120000":
			entry.mode = ENTRY_MODE_SYMLINK
			entry.Type = OBJECT_BLOB
		case "160000":
			entry.mode = ENTRY_MODE_COMMIT
			entry.Type = OBJECT_COMMIT

			step = 8
		case "040000":
			entry.mode = ENTRY_MODE_TREE
			entry.Type = OBJECT_TREE
		default:
			return nil, fmt.Errorf("unknown type: %v", string(data[pos:pos+step]))
		}
		pos += step + 6 // Skip string type of entry type.

		step = 40
		id, err := NewIDFromString(string(data[pos : pos+step]))
		if err != nil {
			return nil, err
		}
		entry.ID = id
		pos += step + 1 // Skip half of sha1.

		step = bytes.IndexByte(data[pos:], '\n')

		// In case entry name is surrounded by double quotes(it happens only in git-shell).
		if data[pos] == '"' {
			entry.name = string(UnescapeChars(data[pos+1 : pos+step-1]))
		} else {
			entry.name = string(data[pos : pos+step])
		}

		pos += step + 1
		entries = append(entries, entry)
	}
	return entries, nil
}

func (t *Tree) SubTree(rpath string) (*Tree, error) {
	if len(rpath) == 0 {
		return t, nil
	}

	paths := strings.Split(rpath, "/")
	var (
		err error
		g   = t
		p   = t
		te  *TreeEntry
	)
	for _, name := range paths {
		te, err = p.GetTreeEntryByPath(name)
		if err != nil {
			return nil, err
		}

		g, err = t.repo.getTree(te.ID)
		if err != nil {
			return nil, err
		}
		g.ptree = p
		p = g
	}
	return g, nil
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries() (Entries, error) {
	if t.entriesParsed {
		return t.entries, nil
	}
	t.entriesParsed = true

	stdout, err := NewCommand("ls-tree", t.ID.String()).RunInDirBytes(t.repo.Path)
	if err != nil {
		return nil, err
	}
	t.entries, err = parseTreeData(t, stdout)
	return t.entries, err
}
