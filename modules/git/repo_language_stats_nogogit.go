// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !gogit
// +build !gogit

package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"math"
	"os"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-enry/go-enry/v2"
)

// GetLanguageStats calculates language stats for git repository at specified commit
func (repo *Repository) GetLanguageStats(commitID string) (map[string]int64, error) {
	// We will feed the commit IDs in order into cat-file --batch, followed by blobs as necessary.
	// so let's create a batch stdin and stdout
	batchStdinWriter, batchReader, cancel := repo.CatFileBatch()
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

	entries, err := tree.ListEntriesRecursive()
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

	contentBuf := bytes.Buffer{}
	var content []byte
	sizes := make(map[string]int64)
	for _, f := range entries {
		contentBuf.Reset()
		content = contentBuf.Bytes()

		if f.Size() == 0 {
			continue
		}

		notVendored := false
		notGenerated := false

		if checker != nil {
			attrs, err := checker.CheckPath(f.Name())
			if err == nil {
				if vendored, has := attrs["linguist-vendored"]; has {
					if vendored == "set" || vendored == "true" {
						continue
					}
					notVendored = vendored == "false"
				}
				if generated, has := attrs["linguist-generated"]; has {
					if generated == "set" || generated == "true" {
						continue
					}
					notGenerated = generated == "false"
				}
				if language, has := attrs["linguist-language"]; has && language != "unspecified" && language != "" {
					// group languages, such as Pug -> HTML; SCSS -> CSS
					group := enry.GetLanguageGroup(language)
					if len(group) != 0 {
						language = group
					}

					sizes[language] += f.Size()
					continue
				}
			}
		}

		if (!notVendored && analyze.IsVendor(f.Name())) || enry.IsDotFile(f.Name()) ||
			enry.IsDocumentation(f.Name()) || enry.IsConfiguration(f.Name()) {
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
			err = discardFull(batchReader, discard)
			if err != nil {
				return nil, err
			}
		}
		if !notGenerated && enry.IsGenerated(f.Name(), content) {
			continue
		}

		// FIXME: Why can't we split this and the IsGenerated tests to avoid reading the blob unless absolutely necessary?
		// - eg. do the all the detection tests using filename first before reading content.
		language := analyze.GetCodeLanguage(f.Name(), content)
		if language == enry.OtherLanguage || language == "" {
			continue
		}

		// group languages, such as Pug -> HTML; SCSS -> CSS
		group := enry.GetLanguageGroup(language)
		if group != "" {
			language = group
		}

		sizes[language] += f.Size()
		continue
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

func discardFull(rd *bufio.Reader, discard int64) error {
	if discard > math.MaxInt32 {
		n, err := rd.Discard(math.MaxInt32)
		discard -= int64(n)
		if err != nil {
			return err
		}
	}
	for discard > 0 {
		n, err := rd.Discard(int(discard))
		discard -= int64(n)
		if err != nil {
			return err
		}
	}
	return nil
}
