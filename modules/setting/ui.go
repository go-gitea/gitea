// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/container"
)

// UI settings
var UI = struct {
	ExplorePagingNum      int
	SitemapPagingNum      int
	IssuePagingNum        int
	RepoSearchPagingNum   int
	MembersPagingNum      int
	FeedMaxCommitNum      int
	FeedPagingNum         int
	PackagesPagingNum     int
	GraphMaxCommitNum     int
	CodeCommentLines      int
	ReactionMaxUserNum    int
	MaxDisplayFileSize    int64
	ShowUserEmail         bool
	DefaultShowFullName   bool
	DefaultTheme          string
	Themes                []string
	Reactions             []string
	ReactionsLookup       container.Set[string] `ini:"-"`
	CustomEmojis          []string
	CustomEmojisMap       map[string]string `ini:"-"`
	SearchRepoDescription bool
	OnlyShowRelevantRepos bool
	ExploreDefaultSort    string `ini:"EXPLORE_PAGING_DEFAULT_SORT"`

	Notification struct {
		MinTimeout            time.Duration
		TimeoutStep           time.Duration
		MaxTimeout            time.Duration
		EventSourceUpdateTime time.Duration
	} `ini:"ui.notification"`

	SVG struct {
		Enabled bool `ini:"ENABLE_RENDER"`
	} `ini:"ui.svg"`

	CSV struct {
		MaxFileSize int64
	} `ini:"ui.csv"`

	Admin struct {
		UserPagingNum   int
		RepoPagingNum   int
		NoticePagingNum int
		OrgPagingNum    int
	} `ini:"ui.admin"`
	User struct {
		RepoPagingNum int
	} `ini:"ui.user"`
	Meta struct {
		Author      string
		Description string
		Keywords    string
	} `ini:"ui.meta"`
}{
	ExplorePagingNum:    20,
	SitemapPagingNum:    20,
	IssuePagingNum:      20,
	RepoSearchPagingNum: 20,
	MembersPagingNum:    20,
	FeedMaxCommitNum:    5,
	FeedPagingNum:       20,
	PackagesPagingNum:   20,
	GraphMaxCommitNum:   100,
	CodeCommentLines:    4,
	ReactionMaxUserNum:  10,
	MaxDisplayFileSize:  8388608,
	DefaultTheme:        `gitea-auto`,
	Themes:              []string{`gitea-auto`, `gitea-light`, `gitea-dark`},
	Reactions:           []string{`+1`, `-1`, `laugh`, `hooray`, `confused`, `heart`, `rocket`, `eyes`},
	CustomEmojis:        []string{`git`, `gitea`, `codeberg`, `gitlab`, `github`, `gogs`},
	CustomEmojisMap:     map[string]string{"git": ":git:", "gitea": ":gitea:", "codeberg": ":codeberg:", "gitlab": ":gitlab:", "github": ":github:", "gogs": ":gogs:"},
	Notification: struct {
		MinTimeout            time.Duration
		TimeoutStep           time.Duration
		MaxTimeout            time.Duration
		EventSourceUpdateTime time.Duration
	}{
		MinTimeout:            10 * time.Second,
		TimeoutStep:           10 * time.Second,
		MaxTimeout:            60 * time.Second,
		EventSourceUpdateTime: 10 * time.Second,
	},
	SVG: struct {
		Enabled bool `ini:"ENABLE_RENDER"`
	}{
		Enabled: true,
	},
	CSV: struct {
		MaxFileSize int64
	}{
		MaxFileSize: 524288,
	},
	Admin: struct {
		UserPagingNum   int
		RepoPagingNum   int
		NoticePagingNum int
		OrgPagingNum    int
	}{
		UserPagingNum:   50,
		RepoPagingNum:   50,
		NoticePagingNum: 25,
		OrgPagingNum:    50,
	},
	User: struct {
		RepoPagingNum int
	}{
		RepoPagingNum: 15,
	},
	Meta: struct {
		Author      string
		Description string
		Keywords    string
	}{
		Author:      "Gitea - Git with a cup of tea",
		Description: "Gitea (Git with a cup of tea) is a painless self-hosted Git service written in Go",
		Keywords:    "go,git,self-hosted,gitea",
	},
}

func loadUIFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "ui", &UI)
	sec := rootCfg.Section("ui")
	UI.ShowUserEmail = sec.Key("SHOW_USER_EMAIL").MustBool(true)
	UI.DefaultShowFullName = sec.Key("DEFAULT_SHOW_FULL_NAME").MustBool(false)
	UI.SearchRepoDescription = sec.Key("SEARCH_REPO_DESCRIPTION").MustBool(true)

	// OnlyShowRelevantRepos=false is important for many private/enterprise instances,
	// because many private repositories do not have "description/topic", users just want to search by their names.
	UI.OnlyShowRelevantRepos = sec.Key("ONLY_SHOW_RELEVANT_REPOS").MustBool(false)

	UI.ReactionsLookup = make(container.Set[string])
	for _, reaction := range UI.Reactions {
		UI.ReactionsLookup.Add(reaction)
	}
	UI.CustomEmojisMap = make(map[string]string)
	for _, emoji := range UI.CustomEmojis {
		UI.CustomEmojisMap[emoji] = ":" + emoji + ":"
	}
}
