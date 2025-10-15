// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
)

// ReadTreeToIndex reads a treeish to the index
func (repo *Repository) ReadTreeToIndex(ctx context.Context, treeish string, indexFilename ...string) error {
	objectFormat, err := repo.GetObjectFormat(ctx)
	if err != nil {
		return err
	}

	if len(treeish) != objectFormat.FullLength() {
		res, _, err := gitcmd.NewCommand("rev-parse", "--verify").AddDynamicArguments(treeish).WithDir(repo.Path).RunStdString(ctx)
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
	return repo.readTreeToIndex(ctx, id, indexFilename...)
}

func (repo *Repository) readTreeToIndex(ctx context.Context, id ObjectID, indexFilename ...string) error {
	var env []string
	if len(indexFilename) > 0 {
		env = append(os.Environ(), "GIT_INDEX_FILE="+indexFilename[0])
	}
	_, _, err := gitcmd.NewCommand("read-tree").AddDynamicArguments(id.String()).WithDir(repo.Path).WithEnv(env).RunStdString(ctx)
	if err != nil {
		return err
	}
	return nil
}

// ReadTreeToTemporaryIndex reads a treeish to a temporary index file
func (repo *Repository) ReadTreeToTemporaryIndex(ctx context.Context, treeish string) (tmpIndexFilename, tmpDir string, cancel context.CancelFunc, err error) {
	defer func() {
		// if error happens and there is a cancel function, do clean up
		if err != nil && cancel != nil {
			cancel()
			cancel = nil
		}
	}()

	tmpDir, cancel, err = setting.AppDataTempDir("git-repo-content").MkdirTempRandom("index")
	if err != nil {
		return "", "", nil, err
	}

	tmpIndexFilename = filepath.Join(tmpDir, ".tmp-index")

	err = repo.ReadTreeToIndex(ctx, treeish, tmpIndexFilename)
	if err != nil {
		return "", "", cancel, err
	}
	return tmpIndexFilename, tmpDir, cancel, nil
}

// EmptyIndex empties the index
func (repo *Repository) EmptyIndex(ctx context.Context) error {
	_, _, err := gitcmd.NewCommand("read-tree", "--empty").WithDir(repo.Path).RunStdString(ctx)
	return err
}

// LsFiles checks if the given filenames are in the index
func (repo *Repository) LsFiles(ctx context.Context, filenames ...string) ([]string, error) {
	cmd := gitcmd.NewCommand("ls-files", "-z").AddDashesAndList(filenames...)
	res, _, err := cmd.WithDir(repo.Path).RunStdBytes(ctx)
	if err != nil {
		return nil, err
	}
	filelist := make([]string, 0, len(filenames))
	for line := range bytes.SplitSeq(res, []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, err
}

// RemoveFilesFromIndex removes given filenames from the index - it does not check whether they are present.
func (repo *Repository) RemoveFilesFromIndex(ctx context.Context, filenames ...string) error {
	objectFormat, err := repo.GetObjectFormat(ctx)
	if err != nil {
		return err
	}
	cmd := gitcmd.NewCommand("update-index", "--remove", "-z", "--index-info")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	buffer := new(bytes.Buffer)
	for _, file := range filenames {
		if file != "" {
			// using format: mode SP type SP sha1 TAB path
			buffer.WriteString("0 blob " + objectFormat.EmptyObjectID().String() + "\t" + file + "\000")
		}
	}
	return cmd.
		WithDir(repo.Path).
		WithStdin(bytes.NewReader(buffer.Bytes())).
		WithStdout(stdout).
		WithStderr(stderr).
		Run(ctx)
}

type IndexObjectInfo struct {
	Mode     string
	Object   ObjectID
	Filename string
}

// AddObjectsToIndex adds the provided object hashes to the index at the provided filenames
func (repo *Repository) AddObjectsToIndex(ctx context.Context, objects ...IndexObjectInfo) error {
	cmd := gitcmd.NewCommand("update-index", "--add", "--replace", "-z", "--index-info")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	buffer := new(bytes.Buffer)
	for _, object := range objects {
		// using format: mode SP type SP sha1 TAB path
		buffer.WriteString(object.Mode + " blob " + object.Object.String() + "\t" + object.Filename + "\000")
	}
	return cmd.
		WithDir(repo.Path).
		WithStdin(bytes.NewReader(buffer.Bytes())).
		WithStdout(stdout).
		WithStderr(stderr).
		Run(ctx)
}

// AddObjectToIndex adds the provided object hash to the index at the provided filename
func (repo *Repository) AddObjectToIndex(ctx context.Context, mode string, object ObjectID, filename string) error {
	return repo.AddObjectsToIndex(ctx, IndexObjectInfo{Mode: mode, Object: object, Filename: filename})
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (repo *Repository) WriteTree(ctx context.Context) (*Tree, error) {
	stdout, _, runErr := gitcmd.NewCommand("write-tree").WithDir(repo.Path).RunStdString(ctx)
	if runErr != nil {
		return nil, runErr
	}
	id, err := NewIDFromString(strings.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}
	return NewTree(repo, id), nil
}
