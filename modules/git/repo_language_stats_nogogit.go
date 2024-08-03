// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bytes"
	"io"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"

	"github.com/go-enry/go-enry/v2"
)

// GetLanguageStats calculates language stats for git repository at specified commit
func (repo *Repository) GetLanguageStats(commitID string) (map[string]int64, error) {
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
	shaBytes, typ, size, err := ReadBatchLine(batchReader)
	if typ != "commit" {
		log.Debug("Unable to get commit for: %s. Err: %v", commitID, err)
		return nil, ErrNotExist{commitID, ""}
	}

	sha, err := NewIDFromString(string(shaBytes))
	if err != nil {
		log.Debug("Unable to get commit for: %s. Err: %v", commitID, err)
		return nil, ErrNotExist{commitID, ""}
	}

	commit, err := CommitFromReader(repo, sha, io.LimitReader(batchReader, size))
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

	checker, deferable := repo.CheckAttributeReader(commitID)
	defer deferable()

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
		isGenerated := optional.None[bool]()
		isDocumentation := optional.None[bool]()
		isDetectable := optional.None[bool]()

		if checker != nil {
			attrs, err := checker.CheckPath(f.Name())
			if err == nil {
				isVendored = AttributeToBool(attrs, AttributeLinguistVendored)
				if isVendored.ValueOrDefault(false) {
					continue
				}

				isGenerated = AttributeToBool(attrs, AttributeLinguistGenerated)
				if isGenerated.ValueOrDefault(false) {
					continue
				}

				isDocumentation = AttributeToBool(attrs, AttributeLinguistDocumentation)
				if isDocumentation.ValueOrDefault(false) {
					continue
				}

				isDetectable = AttributeToBool(attrs, AttributeLinguistDetectable)
				if !isDetectable.ValueOrDefault(true) {
					continue
				}

				hasLanguage := TryReadLanguageAttribute(attrs)
				if hasLanguage.Value() != "" {
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
			_, _, size, err := ReadBatchLine(batchReader)
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
			if err := DiscardFull(batchReader, discard); err != nil {
				return nil, err
			}
		}
		if !isGenerated.Has() && enry.IsGenerated(f.Name(), content) {
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
