// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bytes"
	"io"
	"io/ioutil"

	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
	"github.com/go-enry/go-enry/v2"
)

const fileSizeLimit int64 = 16 * 1024 // 16 KiB
const bigFileSize int64 = 1024 * 1024 // 1 MiB

// GetLanguageStats calculates language stats for git repository at specified commit
func (repo *Repository) GetLanguageStats(commitID string) (map[string]int64, error) {
	// FIXME: We can be more efficient here...
	//
	// We're expecting that we will be reading a lot of blobs and the trees
	// Thus we should use a shared `cat-file --batch` to get all of this data
	// And keep the buffers around with resets as necessary.
	//
	// It's more complicated so...
	commit, err := repo.GetCommit(commitID)
	if err != nil {
		log.Error("Unable to get commit for: %s", commitID)
		return nil, err
	}

	tree := commit.Tree()

	entries, err := tree.ListEntriesRecursive()
	if err != nil {
		return nil, err
	}

	sizes := make(map[string]int64)
	for _, f := range entries {
		if f.Size() == 0 || enry.IsVendor(f.Name()) || enry.IsDotFile(f.Name()) ||
			enry.IsDocumentation(f.Name()) || enry.IsConfiguration(f.Name()) {
			continue
		}

		// If content can not be read or file is too big just do detection by filename
		var content []byte
		if f.Size() <= bigFileSize {
			content, _ = readFile(f, fileSizeLimit)
		}
		if enry.IsGenerated(f.Name(), content) {
			continue
		}

		// TODO: Use .gitattributes file for linguist overrides
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

func readFile(entry service.TreeEntry, limit int64) ([]byte, error) {
	// FIXME: We can probably be a little more efficient here... see above
	r, err := entry.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if limit <= 0 {
		return ioutil.ReadAll(r)
	}

	size := entry.Size()
	if limit > 0 && size > limit {
		size = limit
	}
	buf := bytes.NewBuffer(nil)
	buf.Grow(int(size))
	_, err = io.Copy(buf, io.LimitReader(r, limit))
	return buf.Bytes(), err
}
