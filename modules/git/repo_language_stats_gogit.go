// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build gogit
// +build gogit

package git

import (
	"bytes"
	"context"
	"io"
	"os"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

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

	var checker *CheckAttributeReader

	if CheckGitVersionAtLeast("1.7.8") == nil {
		indexFilename, deleteTemporaryFile, err := repo.ReadTreeToTemporaryIndex(commitID)
		if err == nil {
			defer deleteTemporaryFile()
			tmpWorkTree, err := os.MkdirTemp("", "empty-work-dir")
			if err == nil {
				defer func() {
					_ = util.RemoveAll(tmpWorkTree)
				}()

				checker = &CheckAttributeReader{
					Attributes: []string{"linguist-vendored", "linguist-generated", "linguist-language"},
					Repo:       repo,
					IndexFile:  indexFilename,
					WorkTree:   tmpWorkTree,
				}
				ctx, cancel := context.WithCancel(DefaultContext)
				if err := checker.Init(ctx); err != nil {
					log.Error("Unable to open checker for %s. Error: %v", commitID, err)
				} else {
					go func() {
						err = checker.Run()
						if err != nil {
							log.Error("Unable to open checker for %s. Error: %v", commitID, err)
							cancel()
						}
					}()
				}
				defer cancel()
			}
		}
	}

	sizes := make(map[string]int64)
	err = tree.Files().ForEach(func(f *object.File) error {
		if f.Size == 0 {
			return nil
		}

		notVendored := false
		notGenerated := false

		if checker != nil {
			attrs, err := checker.CheckPath(f.Name)
			if err == nil {
				if vendored, has := attrs["linguist-vendored"]; has {
					if vendored == "set" || vendored == "true" {
						return nil
					}
					notVendored = vendored == "false"
				}
				if generated, has := attrs["linguist-generated"]; has {
					if generated == "set" || generated == "true" {
						return nil
					}
					notGenerated = generated == "false"
				}
				if language, has := attrs["linguist-language"]; has && language != "unspecified" && language != "" {
					// group languages, such as Pug -> HTML; SCSS -> CSS
					group := enry.GetLanguageGroup(language)
					if len(group) != 0 {
						language = group
					}

					sizes[language] += f.Size

					return nil
				}
			}
		}

		if (!notVendored && analyze.IsVendor(f.Name)) || enry.IsDotFile(f.Name) ||
			enry.IsDocumentation(f.Name) || enry.IsConfiguration(f.Name) {
			return nil
		}

		// If content can not be read or file is too big just do detection by filename
		var content []byte
		if f.Size <= bigFileSize {
			content, _ = readFile(f, fileSizeLimit)
		}
		if !notGenerated && enry.IsGenerated(f.Name, content) {
			return nil
		}

		// TODO: Use .gitattributes file for linguist overrides

		language := analyze.GetCodeLanguage(f.Name, content)
		if language == enry.OtherLanguage || language == "" {
			return nil
		}

		// group languages, such as Pug -> HTML; SCSS -> CSS
		group := enry.GetLanguageGroup(language)
		if group != "" {
			language = group
		}

		sizes[language] += f.Size

		return nil
	})
	if err != nil {
		return nil, err
	}

	// filter special languages unless they are the only language
	if len(sizes) > 1 {
		for language := range sizes {
			langtype := enry.GetLanguageType(language)
			if langtype != enry.Programming && langtype != enry.Markup {
				delete(sizes, language)
			}
		}
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
		return io.ReadAll(r)
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
