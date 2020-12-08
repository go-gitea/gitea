// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bytes"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

// ___
//  |  ._   _|  _
// _|_ | | (_| (/_ ><
//

// IndexService represents a native index service
type IndexService struct{}

var _ (service.IndexService) = IndexService{}

// ReadTreeToIndex reads a treeish to the index
func (service IndexService) ReadTreeToIndex(repo service.Repository, treeish string) error {
	if len(treeish) != 40 {
		res, err := git.NewCommand("rev-parse", "--verify", treeish).RunInDir(repo.Path())
		if err != nil {
			return err
		}
		if len(res) > 0 {
			treeish = res[:len(res)-1]
		}
	}
	return service.readTreeToIndex(repo, treeish)
}

func (IndexService) readTreeToIndex(repo service.Repository, id string) error {
	_, err := git.NewCommand("read-tree", id).RunInDir(repo.Path())
	if err != nil {
		return err
	}
	return nil
}

// EmptyIndex empties the index
func (IndexService) EmptyIndex(repo service.Repository) error {
	_, err := git.NewCommand("read-tree", "--empty").RunInDir(repo.Path())
	return err
}

// LsFiles checks if the given filenames are in the index
func (IndexService) LsFiles(repo service.Repository, filenames ...string) ([]string, error) {
	cmd := git.NewCommand("ls-files", "-z", "--")
	for _, arg := range filenames {
		if arg != "" {
			cmd.AddArguments(arg)
		}
	}
	res, err := cmd.RunInDirBytes(repo.Path())
	if err != nil {
		return nil, err
	}
	filelist := make([]string, 0, len(filenames))
	for _, line := range bytes.Split(res, []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, err
}

// RemoveFilesFromIndex removes given filenames from the index - it does not check whether they are present.
func (IndexService) RemoveFilesFromIndex(repo service.Repository, filenames ...string) error {
	cmd := git.NewCommand("update-index", "--remove", "-z", "--index-info")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	buffer := new(bytes.Buffer)
	for _, file := range filenames {
		if file != "" {
			buffer.WriteString("0 0000000000000000000000000000000000000000\t")
			buffer.WriteString(file)
			buffer.WriteByte('\000')
		}
	}
	return cmd.RunInDirFullPipeline(repo.Path(), stdout, stderr, bytes.NewReader(buffer.Bytes()))
}

// AddObjectToIndex adds the provided object hash to the index at the provided filename
func (IndexService) AddObjectToIndex(repo service.Repository, mode string, object service.Hash, filename string) error {
	cmd := git.NewCommand("update-index", "--add", "--replace", "--cacheinfo", mode, object.String(), filename)
	_, err := cmd.RunInDir(repo.Path())
	return err
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (IndexService) WriteTree(repo service.Repository) (service.Tree, error) {
	res, err := git.NewCommand("write-tree").RunInDir(repo.Path())
	if err != nil {
		return nil, err
	}
	id := StringHash(strings.TrimSpace(res))

	tree := &Tree{
		Object: Object{
			hash: id,
			repo: repo,
		},
	}

	return tree, nil
}
