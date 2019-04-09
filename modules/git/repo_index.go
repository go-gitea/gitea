// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// ReadTreeToIndex reads a treeish to the index
func (repo *Repository) ReadTreeToIndex(treeish string) error {
	treeish, err := GetFullCommitID(repo.Path, treeish)
	if err != nil {
		return err
	}

	id, err := NewIDFromString(treeish)
	if err != nil {
		return err
	}
	return repo.readTreeToIndex(id)
}

func (repo *Repository) readTreeToIndex(id SHA1) error {
	_, err := NewCommand("read-tree", id.String()).RunInDir(repo.Path)
	if err != nil {
		return err
	}
	return nil
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

// AddObjectToIndex adds and replaces if necessary the provided object hash to the index at the provided filename
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

// CheckAttribute checks the given attribute of the provided files, use "--all" for all attributes
func (repo *Repository) CheckAttribute(fromIndex bool, attribute string, args ...string) (map[string]map[string]string, error) {
	cmd := NewCommand("check-attr", "-z", attribute)

	if fromIndex {
		cmd.AddArguments("--cached")
	}
	cmd.AddArguments("--")
	for _, arg := range args {
		if arg != "" {
			cmd.AddArguments(arg)
		}
	}

	res, err := cmd.RunInDirBytes(repo.Path)
	if err != nil {
		err = fmt.Errorf("Failed to get attribute %s in repo %s for files %v. Error: %v", attribute, repo.Path, args, err)
		return nil, err
	}

	fields := bytes.Split(res, []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, fmt.Errorf("Wrong number of fields in return from check-attr")
	}

	var name2attribute2info = make(map[string]map[string]string)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := name2attribute2info[filename]
		if attribute2info == nil {
			attribute2info = make(map[string]string)
		}
		attribute2info[attribute] = info
		name2attribute2info[filename] = attribute2info
	}

	return name2attribute2info, err
}

// DiffIndex diffs the current index to a provided tree. It returns a reader of the raw patch
func (repo *Repository) DiffIndex(treeish string) (io.Reader, error) {
	treeish, err := GetFullCommitID(repo.Path, treeish)
	if err != nil {
		return nil, err
	}
	cmd := NewCommand("diff-index", "--cached", "-p", treeish)

	stdoutReader, stdoutWriter := io.Pipe()
	stderr := new(bytes.Buffer)
	err = cmd.RunInDirPipeline(repo.Path, stdoutWriter, stderr)
	return stdoutReader, err
}
