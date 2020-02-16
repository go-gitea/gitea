// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io"
	"io/ioutil"
	"math"
	"path/filepath"

	"github.com/src-d/enry/v2"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

const fileSizeLimit int64 = 16 * 1024 * 1024

// GetLanguageStats calculates language stats for git repository at specified commit
func (repo *Repository) GetLanguageStats(commitID string) (map[string]float32, error) {
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
	var total int64
	err = tree.Files().ForEach(func(f *object.File) error {
		if enry.IsVendor(f.Name) || enry.IsDotFile(f.Name) ||
			enry.IsDocumentation(f.Name) || enry.IsConfiguration(f.Name) {
			return nil
		}

		// TODO: Use .gitattributes file for linguist overrides

		language, ok := enry.GetLanguageByExtension(f.Name)
		if !ok {
			if language, ok = enry.GetLanguageByFilename(f.Name); !ok {
				content, err := readFile(f, fileSizeLimit)
				if err != nil {
					return nil
				}

				language = enry.GetLanguage(filepath.Base(f.Name), content)
				if language == enry.OtherLanguage {
					return nil
				}
			}
		}

		if language != "" {
			sizes[language] += f.Size
			total += f.Size
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	stats := make(map[string]float32)
	var otherPerc float32 = 100
	for language, size := range sizes {
		perc := float32(math.Round(float64(size)/float64(total)*1000) / 10)
		if perc <= 0.1 {
			continue
		}
		otherPerc -= perc
		stats[language] = perc
	}
	otherPerc = float32(math.Round(float64(otherPerc)*10) / 10)
	if otherPerc > 0 {
		stats["other"] = otherPerc
	}
	return stats, nil
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
