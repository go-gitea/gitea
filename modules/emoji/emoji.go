// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2015 Kenneth Shaw
// SPDX-License-Identifier: MIT

package emoji

import (
	"io"
	"sort"
	"strings"
	"sync/atomic"

	"code.gitea.io/gitea/modules/setting"
)

// Gemoji is a set of emoji data.
type Gemoji []Emoji

// Emoji represents a single emoji and associated data.
type Emoji struct {
	Emoji          string
	Description    string
	Aliases        []string
	UnicodeVersion string
	SkinTones      bool
}

type globalVarsStruct struct {
	codeMap       map[string]int    // emoji unicode code to its emoji data.
	aliasMap      map[string]int    // the alias to its emoji data.
	emptyReplacer *strings.Replacer // string replacer for emoji codes, used for finding emoji positions.
	codeReplacer  *strings.Replacer // string replacer for emoji codes.
	aliasReplacer *strings.Replacer // string replacer for emoji aliases.
}

var globalVarsStore atomic.Pointer[globalVarsStruct]

func globalVars() *globalVarsStruct {
	vars := globalVarsStore.Load()
	if vars != nil {
		return vars
	}
	// although there can be concurrent calls, the result should be the same, and there is no performance problem
	vars = &globalVarsStruct{}
	vars.codeMap = make(map[string]int, len(GemojiData))
	vars.aliasMap = make(map[string]int, len(GemojiData))

	// process emoji codes and aliases
	codePairs := make([]string, 0)
	emptyPairs := make([]string, 0)
	aliasPairs := make([]string, 0)

	// sort from largest to small so we match combined emoji first
	sort.Slice(GemojiData, func(i, j int) bool {
		return len(GemojiData[i].Emoji) > len(GemojiData[j].Emoji)
	})

	for idx, emoji := range GemojiData {
		if emoji.Emoji == "" || len(emoji.Aliases) == 0 {
			continue
		}

		// process aliases
		firstAlias := ""
		for _, alias := range emoji.Aliases {
			if alias == "" {
				continue
			}
			enabled := len(setting.UI.EnabledEmojisSet) == 0 || setting.UI.EnabledEmojisSet.Contains(alias)
			if !enabled {
				continue
			}
			if firstAlias == "" {
				firstAlias = alias
			}
			vars.aliasMap[alias] = idx
			aliasPairs = append(aliasPairs, ":"+alias+":", emoji.Emoji)
		}

		// process emoji code
		if firstAlias != "" {
			vars.codeMap[emoji.Emoji] = idx
			codePairs = append(codePairs, emoji.Emoji, ":"+emoji.Aliases[0]+":")
			emptyPairs = append(emptyPairs, emoji.Emoji, emoji.Emoji)
		}
	}

	// create replacers
	vars.emptyReplacer = strings.NewReplacer(emptyPairs...)
	vars.codeReplacer = strings.NewReplacer(codePairs...)
	vars.aliasReplacer = strings.NewReplacer(aliasPairs...)
	globalVarsStore.Store(vars)
	return vars
}

// FromCode retrieves the emoji data based on the provided unicode code (ie,
// "\u2618" will return the Gemoji data for "shamrock").
func FromCode(code string) *Emoji {
	i, ok := globalVars().codeMap[code]
	if !ok {
		return nil
	}

	return &GemojiData[i]
}

// FromAlias retrieves the emoji data based on the provided alias in the form
// "alias" or ":alias:" (ie, "shamrock" or ":shamrock:" will return the Gemoji
// data for "shamrock").
func FromAlias(alias string) *Emoji {
	if strings.HasPrefix(alias, ":") && strings.HasSuffix(alias, ":") {
		alias = alias[1 : len(alias)-1]
	}

	i, ok := globalVars().aliasMap[alias]
	if !ok {
		return nil
	}

	return &GemojiData[i]
}

// ReplaceCodes replaces all emoji codes with the first corresponding emoji
// alias (in the form of ":alias:") (ie, "\u2618" will be converted to
// ":shamrock:").
func ReplaceCodes(s string) string {
	return globalVars().codeReplacer.Replace(s)
}

// ReplaceAliases replaces all aliases of the form ":alias:" with its
// corresponding unicode value.
func ReplaceAliases(s string) string {
	return globalVars().aliasReplacer.Replace(s)
}

type rememberSecondWriteWriter struct {
	pos        int
	idx        int
	end        int
	writecount int
}

func (n *rememberSecondWriteWriter) Write(p []byte) (int, error) {
	n.writecount++
	if n.writecount == 2 {
		n.idx = n.pos
		n.end = n.pos + len(p)
		n.pos += len(p)
		return len(p), io.EOF
	}
	n.pos += len(p)
	return len(p), nil
}

func (n *rememberSecondWriteWriter) WriteString(s string) (int, error) {
	n.writecount++
	if n.writecount == 2 {
		n.idx = n.pos
		n.end = n.pos + len(s)
		n.pos += len(s)
		return len(s), io.EOF
	}
	n.pos += len(s)
	return len(s), nil
}

// FindEmojiSubmatchIndex returns index pair of longest emoji in a string
func FindEmojiSubmatchIndex(s string) []int {
	secondWriteWriter := rememberSecondWriteWriter{}

	// A faster and clean implementation would copy the trie tree formation in strings.NewReplacer but
	// we can be lazy here.
	//
	// The implementation of strings.Replacer.WriteString is such that the first index of the emoji
	// submatch is simply the second thing that is written to WriteString in the writer.
	//
	// Therefore we can simply take the index of the second write as our first emoji
	//
	// FIXME: just copy the trie implementation from strings.NewReplacer
	_, _ = globalVars().emptyReplacer.WriteString(&secondWriteWriter, s)

	// if we wrote less than twice then we never "replaced"
	if secondWriteWriter.writecount < 2 {
		return nil
	}

	return []int{secondWriteWriter.idx, secondWriteWriter.end}
}
