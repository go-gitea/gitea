// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"errors"
	"os"
	"path/filepath"
	"unicode"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/blevesearch/bleve/v2"
	unicode_tokenizer "github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
)

const (
	maxFuzziness          = 2
	indexMetadataFilename = "rupture_meta.json" // named after the former "rupture" dependency, renaming it would force a reindex
)

type indexMetadata struct {
	Version int `json:"version"`
}

func readIndexMetadataVersion(path string) (int, error) {
	data, err := os.ReadFile(filepath.Join(path, indexMetadataFilename))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	var metadata indexMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return 0, err
	}
	return metadata.Version, nil
}

func writeIndexMetadataVersion(path string, version int) error {
	data, _ := json.Marshal(indexMetadata{Version: version}) // marshalling a single int cannot fail
	return os.WriteFile(filepath.Join(path, indexMetadataFilename), data, 0o666)
}

// openIndexer open the index at the specified path, checking for metadata
// updates and bleve version updates.  If index needs to be created (or
// re-created), returns (nil, nil)
func openIndexer(path string, latestVersion int) (bleve.Index, int, error) {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return nil, 0, nil
	} else if err != nil {
		return nil, 0, err
	}

	version, err := readIndexMetadataVersion(path)
	if err != nil {
		return nil, 0, err
	}
	if version < latestVersion {
		// the indexer is using a previous version, so we should delete it and
		// re-populate
		return nil, version, os.RemoveAll(path)
	}

	index, err := bleve.Open(path)
	if err != nil {
		if errors.Is(err, upsidedown.IncompatibleVersion) {
			log.Warn("Indexer was built with a previous version of bleve, deleting and rebuilding")
			return nil, 0, os.RemoveAll(path)
		}
		return nil, 0, err
	}

	return index, 0, nil
}

// GuessFuzzinessByKeyword guesses fuzziness based on the levenshtein distance and determines how many chars
// may be different on two string, and they still be considered equivalent.
// Given a phrase, its shortest word determines its fuzziness. If a phrase uses CJK (eg: `갃갃갃` `啊啊啊`), the fuzziness is zero.
func GuessFuzzinessByKeyword(s string) int {
	tokenizer := unicode_tokenizer.NewUnicodeTokenizer()
	tokens := tokenizer.Tokenize([]byte(s))

	if len(tokens) > 0 {
		fuzziness := maxFuzziness

		for _, token := range tokens {
			fuzziness = min(fuzziness, guessFuzzinessByKeyword(string(token.Term)))
		}

		return fuzziness
	}

	return 0
}

func guessFuzzinessByKeyword(s string) int {
	// according to https://github.com/blevesearch/bleve/issues/1563, the supported max fuzziness is 2
	// magic number 4 was chosen to determine the levenshtein distance per each character of a keyword
	// BUT, when using CJK (eg: `갃갃갃` `啊啊啊`), it mismatches a lot.
	// Likewise, queries whose terms contains characters that are *not* letters should not use fuzziness

	for _, r := range s {
		if r >= 128 || !unicode.IsLetter(r) {
			return 0
		}
	}
	return min(min(setting.Indexer.TypeBleveMaxFuzzniess, maxFuzziness), len(s)/4)
}
