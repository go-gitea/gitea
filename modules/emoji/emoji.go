// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2015 Kenneth Shaw
// SPDX-License-Identifier: MIT

package emoji

import (
	"sort"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/public"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// Emoji represents a single emoji and associated data.
type Emoji struct {
	Emoji   string   `json:"emoji"`
	Aliases []string `json:"aliases"`
}

type globalVarsStruct struct {
	emojis         []Emoji
	codeMap        map[string]int    // emoji Unicode code to its emoji data.
	aliasMap       map[string]int    // the alias to its emoji data.
	trie           *util.TrieNode    // trie for finding emoji positions.
	isStartingByte [256]bool         // fast-path skip for starting bytes.
	codeReplacer   *strings.Replacer // string replacer for emoji codes.
	aliasReplacer  *strings.Replacer // string replacer for emoji aliases.
}

var globalVarsStore atomic.Pointer[globalVarsStruct]

func globalVars() *globalVarsStruct {
	vars := globalVarsStore.Load()
	if vars != nil {
		return vars
	}
	// although there can be concurrent calls, the result should be the same, and there is no performance problem
	vars = &globalVarsStruct{}
	if data, err := public.AssetFS().ReadFile("assets/emoji.json"); err != nil {
		log.Error("Unable to read assets/emoji.json: %v", err)
	} else if err = json.Unmarshal(data, &vars.emojis); err != nil {
		log.Error("Unable to parse assets/emoji.json: %v", err)
	}
	vars.codeMap = make(map[string]int, len(vars.emojis))
	vars.aliasMap = make(map[string]int, len(vars.emojis))
	vars.trie = &util.TrieNode{}

	// process emoji codes and aliases
	codePairs := make([]string, 0)
	aliasPairs := make([]string, 0)

	// sort from largest to small so we match combined emoji first
	sort.Slice(vars.emojis, func(i, j int) bool {
		return len(vars.emojis[i].Emoji) > len(vars.emojis[j].Emoji)
	})

	for idx, emoji := range vars.emojis {
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
			vars.trie.Insert(emoji.Emoji)
			vars.isStartingByte[emoji.Emoji[0]] = true
		}
	}

	// create replacers
	vars.codeReplacer = strings.NewReplacer(codePairs...)
	vars.aliasReplacer = strings.NewReplacer(aliasPairs...)
	globalVarsStore.Store(vars)
	return vars
}

// FromCode retrieves the emoji data based on the provided Unicode code
// e.g.: "\u2618" will return the emoji data for "shamrock".
func FromCode(code string) *Emoji {
	vars := globalVars()
	i, ok := vars.codeMap[code]
	if !ok {
		i, ok = vars.codeMap[strings.Map(func(r rune) rune { return util.Iif(isSkinTone(r), -1, r) }, code)]
	}
	if !ok {
		return nil
	}

	return &vars.emojis[i]
}

// FromAlias retrieves the emoji data based on the provided alias in the form "alias" or ":alias:"
// e.g.: "shamrock" or ":shamrock:" will return the emoji data for "shamrock".
func FromAlias(alias string) *Emoji {
	if strings.HasPrefix(alias, ":") && strings.HasSuffix(alias, ":") {
		alias = alias[1 : len(alias)-1]
	}

	vars := globalVars()
	i, ok := vars.aliasMap[alias]
	if !ok {
		return nil
	}

	return &vars.emojis[i]
}

// ReplaceCodes replaces all emoji codes with the first corresponding emoji alias in the form of ":alias:"
// e.g.: "\u2618" will be converted to ":shamrock:".
func ReplaceCodes(s string) string {
	return globalVars().codeReplacer.Replace(s)
}

// ReplaceAliases replaces all aliases of the form ":alias:" with its corresponding Unicode value.
func ReplaceAliases(s string) string {
	return globalVars().aliasReplacer.Replace(s)
}

// FindEmojiSubmatchIndex returns index-pair of the first emoji in a string
func FindEmojiSubmatchIndex(s string) []int {
	vars := globalVars()
	for i := 0; i < len(s); i++ {
		if !vars.isStartingByte[s[i]] {
			continue
		}
		if matchLen := vars.trie.Match(s, i, skinToneLen); matchLen > 0 {
			return []int{i, i + matchLen}
		}
	}
	return nil
}

func isSkinTone(r rune) bool {
	return r >= 0x1f3fb && r <= 0x1f3ff
}

// skinToneLen lets skin-toned text match the toneless emoji data
func skinToneLen(s string, pos int) int {
	if s[pos] != 0xf0 {
		return 0
	}
	if r, size := utf8.DecodeRuneInString(s[pos:]); isSkinTone(r) {
		return size
	}
	return 0
}
