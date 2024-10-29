// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// ReadTreeToIndex reads a treeish to the index
func (repo *Repository) ReadTreeToIndex(treeish string, indexFilename ...string) error {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return err
	}

	if len(treeish) != objectFormat.FullLength() {
		res, _, err := NewCommand(repo.Ctx, "rev-parse", "--verify").AddDynamicArguments(treeish).RunStdString(&RunOpts{Dir: repo.Path})
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

func (repo *Repository) readTreeToIndex(id ObjectID, indexFilename ...string) error {
	var env []string
	if len(indexFilename) > 0 {
		env = append(os.Environ(), "GIT_INDEX_FILE="+indexFilename[0])
	}
	_, _, err := NewCommand(repo.Ctx, "read-tree").AddDynamicArguments(id.String()).RunStdString(&RunOpts{Dir: repo.Path, Env: env})
	if err != nil {
		return err
	}
	return nil
}

// ReadTreeToTemporaryIndex reads a treeish to a temporary index file
func (repo *Repository) ReadTreeToTemporaryIndex(treeish string) (tmpIndexFilename, tmpDir string, cancel context.CancelFunc, err error) {
	defer func() {
		// if error happens and there is a cancel function, do clean up
		if err != nil && cancel != nil {
			cancel()
			cancel = nil
		}
	}()

	removeDirFn := func(dir string) func() { // it can't use the return value "tmpDir" directly because it is empty when error occurs
		return func() {
			if err := util.RemoveAll(dir); err != nil {
				log.Error("failed to remove tmp index dir: %v", err)
			}
		}
	}

	tmpDir, err = os.MkdirTemp("", "index")
	if err != nil {
		return "", "", nil, err
	}

	tmpIndexFilename = filepath.Join(tmpDir, ".tmp-index")
	cancel = removeDirFn(tmpDir)
	err = repo.ReadTreeToIndex(treeish, tmpIndexFilename)
	if err != nil {
		return "", "", cancel, err
	}
	return tmpIndexFilename, tmpDir, cancel, err
}

// EmptyIndex empties the index
func (repo *Repository) EmptyIndex() error {
	_, _, err := NewCommand(repo.Ctx, "read-tree", "--empty").RunStdString(&RunOpts{Dir: repo.Path})
	return err
}

// LsFiles checks if the given filenames are in the index
func (repo *Repository) LsFiles(filenames ...string) ([]string, error) {
	cmd := NewCommand(repo.Ctx, "ls-files", "-z").AddDashesAndList(filenames...)
	res, _, err := cmd.RunStdBytes(&RunOpts{Dir: repo.Path})
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
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return err
	}
	cmd := NewCommand(repo.Ctx, "update-index", "--remove", "-z", "--index-info")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	buffer := new(bytes.Buffer)
	for _, file := range filenames {
		if file != "" {
			// using format: mode SP type SP sha1 TAB path
			buffer.WriteString("0 blob " + objectFormat.EmptyObjectID().String() + "\t" + file + "\000")
		}
	}
	return cmd.Run(&RunOpts{
		Dir:    repo.Path,
		Stdin:  bytes.NewReader(buffer.Bytes()),
		Stdout: stdout,
		Stderr: stderr,
	})
}

type IndexObjectInfo struct {
	Mode     string
	Object   ObjectID
	Filename string
}

// AddObjectsToIndex adds the provided object hashes to the index at the provided filenames
func (repo *Repository) AddObjectsToIndex(objects ...IndexObjectInfo) error {
	cmd := NewCommand(repo.Ctx, "update-index", "--add", "--replace", "-z", "--index-info")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	buffer := new(bytes.Buffer)
	for _, object := range objects {
		// using format: mode SP type SP sha1 TAB path
		buffer.WriteString(object.Mode + " blob " + object.Object.String() + "\t" + object.Filename + "\000")
	}
	return cmd.Run(&RunOpts{
		Dir:    repo.Path,
		Stdin:  bytes.NewReader(buffer.Bytes()),
		Stdout: stdout,
		Stderr: stderr,
	})
}

// AddObjectToIndex adds the provided object hash to the index at the provided filename
func (repo *Repository) AddObjectToIndex(mode string, object ObjectID, filename string) error {
	return repo.AddObjectsToIndex(IndexObjectInfo{Mode: mode, Object: object, Filename: filename})
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (repo *Repository) WriteTree() (*Tree, error) {
	stdout, _, runErr := NewCommand(repo.Ctx, "write-tree").RunStdString(&RunOpts{Dir: repo.Path})
	if runErr != nil {
		return nil, runErr
	}
	id, err := NewIDFromString(strings.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}
	return NewTree(repo, id), nil
}
