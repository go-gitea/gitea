// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io"
	"io/ioutil"

	"code.gitea.io/gitea/modules/analyze"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const fileSizeLimit int64 = 16 * 1024 * 1024

// GetLanguageStats calculates language stats for git repository at specified commit
func (repo *Repository) GetLanguageStats(commitID string) (map[string]int64, error) {
	r, err := git.PlainOpen(repo.Path)
	if err != nil {
		return nil, err
	}

	rev, err := r.ResolveRevision(plumbing.Revision(commitID))
	if err != nil {
		return nil, err
	}

	commit, err := r.CommitObject(*rev)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	sizes := make(map[string]int64)
	err = tree.Files().ForEach(func(f *object.File) error {
		if enry.IsVendor(f.Name) || enry.IsDotFile(f.Name) ||
			enry.IsDocumentation(f.Name) || enry.IsConfiguration(f.Name) {
			return nil
		}

		// If content can not be read just do detection by filename
		content, _ := readFile(f, fileSizeLimit)
		if enry.IsGenerated(f.Name, content) {
			return nil
		}

		// TODO: Use .gitattributes file for linguist overrides

		language := analyze.GetCodeLanguage(f.Name, content)
		if language == enry.OtherLanguage || language == "" {
			language = "other"
		}

		sizes[language] += f.Size

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(sizes) == 0 {
		sizes["other"] = 0
	}

	return sizes, nil
}

func readFile(f *object.File, limit int64) ([]byte, error) {
	r, err := f.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if limit <= 0 {
		return ioutil.ReadAll(r)
	}

	size := f.Size
	if limit > 0 && size > limit {
		size = limit
	}
	buf := bytes.NewBuffer(nil)
	buf.Grow(int(size))
	_, err = io.Copy(buf, io.LimitReader(r, limit))
	return buf.Bytes(), err
}
