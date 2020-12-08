// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// IndexService represents a service that managing indexes in repositories
type IndexService interface {

	// ReadTreeToIndex reads a treeish to the index
	ReadTreeToIndex(repo Repository, treeish string) error

	// EmptyIndex empties the index
	EmptyIndex(repo Repository) error

	// LsFiles checks if the given filenames are in the index
	LsFiles(repo Repository, filenames ...string) ([]string, error)

	// RemoveFilesFromIndex removes given filenames from the index - it does not check whether they are present.
	RemoveFilesFromIndex(repo Repository, filenames ...string) error

	// AddObjectToIndex adds the provided object hash to the index at the provided filename
	AddObjectToIndex(repo Repository, mode string, object Hash, filename string) error

	// WriteTree writes the current index as a tree to the object db and returns its hash
	WriteTree(repo Repository) (Tree, error)
}
