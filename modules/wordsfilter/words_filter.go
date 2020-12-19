// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wordsfilter

import (
	"bufio"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/killtw/lemonade/lemonade"
)

// Search search words on src and return matches
func Search(src string) []string {
	return lemonade.Trie.Search(src)
}

// Replace replace the matched words
func Replace(src string) string {
	if !setting.WordsFilter.Enabled {
		return src
	}
	replaced, _ := lemonade.Replace(src)
	return replaced
}

// Init initialize words filter
func Init() error {
	if !setting.WordsFilter.Enabled {
		return nil
	}

	if err := lemonade.InitTrie(); err != nil {
		return err
	}

	f, err := os.Open(setting.WordsFilter.Filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lemonade.Add(strings.TrimSpace(scanner.Text()))
	}
	return nil
}
