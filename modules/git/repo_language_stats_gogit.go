// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"bytes"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/analyze"

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

	checker, deferable := repo.CheckAttributeReader(commitID)
	defer deferable()

	// sizes contains the current calculated size of all files by language
	sizes := make(map[string]int64)
	// by default we will only count the sizes of programming languages or markup languages
	// unless they are explicitly set using linguist-language
	includedLanguage := map[string]bool{}
	// or if there's only one language in the repository
	firstExcludedLanguage := ""
	firstExcludedLanguageSize := int64(0)

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

					// this language will always be added to the size
					sizes[language] += f.Size
					return nil
				} else if language, has := attrs["gitlab-language"]; has && language != "unspecified" && language != "" {
					// strip off a ? if present
					if idx := strings.IndexByte(language, '?'); idx >= 0 {
						language = language[:idx]
					}
					if len(language) != 0 {
						// group languages, such as Pug -> HTML; SCSS -> CSS
						group := enry.GetLanguageGroup(language)
						if len(group) != 0 {
							language = group
						}

						// this language will always be added to the size
						sizes[language] += f.Size
						return nil
					}
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

		included, checked := includedLanguage[language]
		if !checked {
			langtype := enry.GetLanguageType(language)
			included = langtype == enry.Programming || langtype == enry.Markup
			includedLanguage[language] = included
		}
		if included {
			sizes[language] += f.Size
		} else if len(sizes) == 0 && (firstExcludedLanguage == "" || firstExcludedLanguage == language) {
			firstExcludedLanguage = language
			firstExcludedLanguageSize += f.Size
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// If there are no included languages add the first excluded language
	if len(sizes) == 0 && firstExcludedLanguage != "" {
		sizes[firstExcludedLanguage] = firstExcludedLanguageSize
	}

	return mergeLanguageStats(sizes), nil
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
