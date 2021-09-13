// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// ReadTreeToIndex reads a treeish to the index
func (repo *Repository) ReadTreeToIndex(treeish string, indexFilename ...string) error {
	if len(treeish) != 40 {
		res, err := NewCommand("rev-parse", "--verify", treeish).RunInDir(repo.Path)
		if err != nil {
			return err
		}
		if len(res) > 0 {
			treeish = res[:len(res)-1]
		}
	}
	id, err := NewIDFromString(treeish)
	if err != nil {
		return err
	}
	return repo.readTreeToIndex(id, indexFilename...)
}

func (repo *Repository) readTreeToIndex(id SHA1, indexFilename ...string) error {
	var env []string
	if len(indexFilename) > 0 {
		env = append(os.Environ(), "GIT_INDEX_FILE="+indexFilename[0])
	}
	_, err := NewCommand("read-tree", id.String()).RunInDirWithEnv(repo.Path, env)
	if err != nil {
		return err
	}
	return nil
}

// ReadTreeToTemporaryIndex reads a treeish to a temporary index file
func (repo *Repository) ReadTreeToTemporaryIndex(treeish string) (filename string, cancel context.CancelFunc, err error) {
	tmpIndex, err := ioutil.TempFile("", "index")
	if err != nil {
		return
	}
	filename = tmpIndex.Name()
	cancel = func() {
		err := util.Remove(filename)
		if err != nil {
			log.Error("failed to remove tmp index file: %v", err)
		}
	}
	err = repo.ReadTreeToIndex(treeish, filename)
	if err != nil {
		defer cancel()
		return "", func() {}, err
	}
	return
}

// EmptyIndex empties the index
func (repo *Repository) EmptyIndex() error {
	_, err := NewCommand("read-tree", "--empty").RunInDir(repo.Path)
	return err
}

// LsFiles checks if the given filenames are in the index
func (repo *Repository) LsFiles(filenames ...string) ([]string, error) {
	cmd := NewCommand("ls-files", "-z", "--")
	for _, arg := range filenames {
		if arg != "" {
			cmd.AddArguments(arg)
		}
	}
	res, err := cmd.RunInDirBytes(repo.Path)
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
func (repo *Repository) RemoveFilesFromIndex(filenames ...string) error {
	cmd := NewCommand("update-index", "--remove", "-z", "--index-info")
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
	return cmd.RunInDirFullPipeline(repo.Path, stdout, stderr, bytes.NewReader(buffer.Bytes()))
}

// AddObjectToIndex adds the provided object hash to the index at the provided filename
func (repo *Repository) AddObjectToIndex(mode string, object SHA1, filename string) error {
	cmd := NewCommand("update-index", "--add", "--replace", "--cacheinfo", mode, object.String(), filename)
	_, err := cmd.RunInDir(repo.Path)
	return err
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (repo *Repository) WriteTree() (*Tree, error) {
	res, err := NewCommand("write-tree").RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}
	id, err := NewIDFromString(strings.TrimSpace(res))
	if err != nil {
		return nil, err
	}
	return NewTree(repo, id), nil
}
