// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package languagestats

import (
	"bytes"
	"io"

	"code.gitea.io/gitea/modules/analyze"
	git_module "code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/attribute"
	"code.gitea.io/gitea/modules/optional"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetLanguageStats calculates language stats for git repository at specified commit
func GetLanguageStats(repo *git_module.Repository, commitID string) (map[string]int64, error) {
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

	checker, err := attribute.NewBatchChecker(repo, commitID, attribute.LinguistAttributes)
	if err != nil {
		return nil, err
	}
	defer checker.Close()

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

		isVendored := optional.None[bool]()
		isGenerated := optional.None[bool]()
		isDocumentation := optional.None[bool]()
		isDetectable := optional.None[bool]()

		attrs, err := checker.CheckPath(f.Name)
		if err == nil {
			isVendored = attrs.GetVendored()
			if isVendored.ValueOrDefault(false) {
				return nil
			}

			isGenerated = attrs.GetGenerated()
			if isGenerated.ValueOrDefault(false) {
				return nil
			}

			isDocumentation = attrs.GetDocumentation()
			if isDocumentation.ValueOrDefault(false) {
				return nil
			}

			isDetectable = attrs.GetDetectable()
			if !isDetectable.ValueOrDefault(true) {
				return nil
			}

			hasLanguage := attrs.GetLanguage()
			if hasLanguage.Value() != "" {
				language := hasLanguage.Value()

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

		if (!isVendored.Has() && analyze.IsVendor(f.Name)) ||
			enry.IsDotFile(f.Name) ||
			(!isDocumentation.Has() && enry.IsDocumentation(f.Name)) ||
			enry.IsConfiguration(f.Name) {
			return nil
		}

		// If content can not be read or file is too big just do detection by filename
		var content []byte
		if f.Size <= bigFileSize {
			content, _ = readFile(f, fileSizeLimit)
		}
		if !isGenerated.Has() && enry.IsGenerated(f.Name, content) {
			return nil
		}

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
		if included || isDetectable.ValueOrDefault(false) {
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
