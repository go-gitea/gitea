// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package indexer

type SearchModeType string

const (
	SearchModeExact  SearchModeType = "exact"
	SearchModeWords  SearchModeType = "words"
	SearchModeFuzzy  SearchModeType = "fuzzy"
	SearchModeRegexp SearchModeType = "regexp"
)

type SearchMode struct {
	ModeValue    SearchModeType
	TooltipTrKey string
	TitleTrKey   string
}

func SearchModesExactWords() []SearchMode {
	return []SearchMode{
		{
			ModeValue:    SearchModeExact,
			TooltipTrKey: "search.exact_tooltip",
			TitleTrKey:   "search.exact",
		},
		{
			ModeValue:    SearchModeWords,
			TooltipTrKey: "search.words_tooltip",
			TitleTrKey:   "search.words",
		},
	}
}

func SearchModesExactWordsFuzzy() []SearchMode {
	return append(SearchModesExactWords(), []SearchMode{
		{
			ModeValue:    SearchModeFuzzy,
			TooltipTrKey: "search.fuzzy_tooltip",
			TitleTrKey:   "search.fuzzy",
		},
	}...)
}

func GitGrepSupportedSearchModes() []SearchMode {
	return append(SearchModesExactWords(), []SearchMode{
		{
			ModeValue:    SearchModeRegexp,
			TooltipTrKey: "search.regexp_tooltip",
			TitleTrKey:   "search.regexp",
		},
	}...)
}
