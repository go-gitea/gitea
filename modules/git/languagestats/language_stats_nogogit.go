// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package languagestats

import (
	"bytes"
	"io"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/attribute"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"

	"github.com/go-enry/go-enry/v2"
)

// GetLanguageStats calculates language stats for git repository at specified commit
func GetLanguageStats(repo *git.Repository, commitID string) (map[string]int64, error) {
	// We will feed the commit IDs in order into cat-file --batch, followed by blobs as necessary.
	// so let's create a batch stdin and stdout
	batchStdinWriter, batchReader, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	writeID := func(id string) error {
		_, err := batchStdinWriter.Write([]byte(id + "\n"))
		return err
	}

	if err := writeID(commitID); err != nil {
		return nil, err
	}
	shaBytes, typ, size, err := git.ReadBatchLine(batchReader)
	if typ != "commit" {
		log.Debug("Unable to get commit for: %s. Err: %v", commitID, err)
		return nil, git.ErrNotExist{ID: commitID}
	}

	sha, err := git.NewIDFromString(string(shaBytes))
	if err != nil {
		log.Debug("Unable to get commit for: %s. Err: %v", commitID, err)
		return nil, git.ErrNotExist{ID: commitID}
	}

	commit, err := git.CommitFromReader(repo, sha, io.LimitReader(batchReader, size))
	if err != nil {
		log.Debug("Unable to get commit for: %s. Err: %v", commitID, err)
		return nil, err
	}
	if _, err = batchReader.Discard(1); err != nil {
		return nil, err
	}

	tree := commit.Tree

	entries, err := tree.ListEntriesRecursiveWithSize()
	if err != nil {
		return nil, err
	}

	checker, err := attribute.NewBatchChecker(repo, commitID, attribute.LinguistAttributes)
	if err != nil {
		return nil, err
	}
	defer checker.Close()

	contentBuf := bytes.Buffer{}
	var content []byte

	// sizes contains the current calculated size of all files by language
	sizes := make(map[string]int64)
	// by default we will only count the sizes of programming languages or markup languages
	// unless they are explicitly set using linguist-language
	includedLanguage := map[string]bool{}
	// or if there's only one language in the repository
	firstExcludedLanguage := ""
	firstExcludedLanguageSize := int64(0)

	for _, f := range entries {
		select {
		case <-repo.Ctx.Done():
			return sizes, repo.Ctx.Err()
		default:
		}

		contentBuf.Reset()
		content = contentBuf.Bytes()

		if f.Size() == 0 {
			continue
		}

		isVendored := optional.None[bool]()
		isDocumentation := optional.None[bool]()
		isDetectable := optional.None[bool]()

		attrs, err := checker.CheckPath(f.Name())
		attrLinguistGenerated := optional.None[bool]()
		if err == nil {
			if isVendored = attrs.GetVendored(); isVendored.ValueOrDefault(false) {
				continue
			}

			if attrLinguistGenerated = attrs.GetGenerated(); attrLinguistGenerated.ValueOrDefault(false) {
				continue
			}

			if isDocumentation = attrs.GetDocumentation(); isDocumentation.ValueOrDefault(false) {
				continue
			}

			if isDetectable = attrs.GetDetectable(); !isDetectable.ValueOrDefault(true) {
				continue
			}

			if hasLanguage := attrs.GetLanguage(); hasLanguage.Value() != "" {
				language := hasLanguage.Value()

				// group languages, such as Pug -> HTML; SCSS -> CSS
				group := enry.GetLanguageGroup(language)
				if len(group) != 0 {
					language = group
				}

				// this language will always be added to the size
				sizes[language] += f.Size()
				continue
			}
		}

		if (!isVendored.Has() && analyze.IsVendor(f.Name())) ||
			enry.IsDotFile(f.Name()) ||
			(!isDocumentation.Has() && enry.IsDocumentation(f.Name())) ||
			enry.IsConfiguration(f.Name()) {
			continue
		}

		// If content can not be read or file is too big just do detection by filename

		if f.Size() <= bigFileSize {
			if err := writeID(f.ID.String()); err != nil {
				return nil, err
			}
			_, _, size, err := git.ReadBatchLine(batchReader)
			if err != nil {
				log.Debug("Error reading blob: %s Err: %v", f.ID.String(), err)
				return nil, err
			}

			sizeToRead := size
			discard := int64(1)
			if size > fileSizeLimit {
				sizeToRead = fileSizeLimit
				discard = size - fileSizeLimit + 1
			}

			_, err = contentBuf.ReadFrom(io.LimitReader(batchReader, sizeToRead))
			if err != nil {
				return nil, err
			}
			content = contentBuf.Bytes()
			if err := git.DiscardFull(batchReader, discard); err != nil {
				return nil, err
			}
		}

		// if "generated" attribute is set, use it, otherwise use enry.IsGenerated to guess
		var isGenerated bool
		if attrLinguistGenerated.Has() {
			isGenerated = attrLinguistGenerated.Value()
		} else {
			isGenerated = enry.IsGenerated(f.Name(), content)
		}
		if isGenerated {
			continue
		}

		// FIXME: Why can't we split this and the IsGenerated tests to avoid reading the blob unless absolutely necessary?
		// - eg. do the all the detection tests using filename first before reading content.
		language := analyze.GetCodeLanguage(f.Name(), content)
		if language == "" {
			continue
		}

		// group languages, such as Pug -> HTML; SCSS -> CSS
		group := enry.GetLanguageGroup(language)
		if group != "" {
			language = group
		}

		included, checked := includedLanguage[language]
		if !checked {
			langType := enry.GetLanguageType(language)
			included = langType == enry.Programming || langType == enry.Markup
			includedLanguage[language] = included
		}
		if included || isDetectable.ValueOrDefault(false) {
			sizes[language] += f.Size()
		} else if len(sizes) == 0 && (firstExcludedLanguage == "" || firstExcludedLanguage == language) {
			firstExcludedLanguage = language
			firstExcludedLanguageSize += f.Size()
		}
	}

	// If there are no included languages add the first excluded language
	if len(sizes) == 0 && firstExcludedLanguage != "" {
		sizes[firstExcludedLanguage] = firstExcludedLanguageSize
	}

	return mergeLanguageStats(sizes), nil
}
