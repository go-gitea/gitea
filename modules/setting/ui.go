// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
)

// UI settings
var (
	UI = struct {
		GraphMaxCommitNum       int
		ReactionMaxUserNum      int
		MaxDisplayFileSize      int64
		DefaultShowFullName     bool
		DefaultTheme            string
		Themes                  []string
		FileIconTheme           string
		Reactions               []string
		ReactionsLookup         container.Set[string] `ini:"-"`
		CustomEmojis            []string
		CustomEmojisMap         map[string]string `ini:"-"`
		PreferredTimestampTense string

		AmbiguousUnicodeDetection bool

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
			MaxRows     int
		} `ini:"ui.csv"`

		Admin struct {
			UserPagingNum   int
			RepoPagingNum   int
			NoticePagingNum int
			OrgPagingNum    int
		} `ini:"ui.admin"`
		User struct {
			RepoPagingNum int
			OrgPagingNum  int
		} `ini:"ui.user"`
		Meta struct {
			Author      string
			Description string
			Keywords    string
		} `ini:"ui.meta"`
	}{
		GraphMaxCommitNum:       100,
		ReactionMaxUserNum:      10,
		MaxDisplayFileSize:      8388608,
		DefaultTheme:            `gitea-auto`,
		FileIconTheme:           `material`,
		Reactions:               []string{`+1`, `-1`, `laugh`, `hooray`, `confused`, `heart`, `rocket`, `eyes`},
		CustomEmojis:            []string{`git`, `gitea`, `codeberg`, `gitlab`, `github`, `gogs`},
		CustomEmojisMap:         map[string]string{"git": ":git:", "gitea": ":gitea:", "codeberg": ":codeberg:", "gitlab": ":gitlab:", "github": ":github:", "gogs": ":gogs:"},
		PreferredTimestampTense: "mixed",

		AmbiguousUnicodeDetection: true,

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
			MaxRows     int
		}{
			MaxFileSize: 524288,
			MaxRows:     2500,
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
			OrgPagingNum  int
		}{
			RepoPagingNum: 15,
			OrgPagingNum:  15,
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

	ExplorePagingNum      int    // Depreciated: migrated to database
	SitemapPagingNum      int    // Depreciated: migrated to database
	IssuePagingNum        int    // Depreciated: migrated to database
	RepoSearchPagingNum   int    // Depreciated: migrated to database
	MembersPagingNum      int    // Depreciated: migrated to database
	FeedMaxCommitNum      int    // Depreciated: migrated to database
	FeedPagingNum         int    // Depreciated: migrated to database
	PackagesPagingNum     int    // Depreciated: migrated to database
	CodeCommentLines      int    // Depreciated: migrated to database
	ShowUserEmail         bool   // Depreciated: migrated to database
	SearchRepoDescription bool   // Depreciated: migrated to database
	OnlyShowRelevantRepos bool   // Depreciated: migrated to database
	ExploreDefaultSort    string // Depreciated: migrated to database
)

func loadUIFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "ui", &UI)
	sec := rootCfg.Section("ui")
	UI.DefaultShowFullName = sec.Key("DEFAULT_SHOW_FULL_NAME").MustBool(false)

	if UI.PreferredTimestampTense != "mixed" && UI.PreferredTimestampTense != "absolute" {
		log.Fatal("ui.PREFERRED_TIMESTAMP_TENSE must be either 'mixed' or 'absolute'")
	}

	UI.ReactionsLookup = make(container.Set[string])
	for _, reaction := range UI.Reactions {
		UI.ReactionsLookup.Add(reaction)
	}
	UI.CustomEmojisMap = make(map[string]string)
	for _, emoji := range UI.CustomEmojis {
		UI.CustomEmojisMap[emoji] = ":" + emoji + ":"
	}

	ExplorePagingNum = sec.Key("EXPLORE_PAGING_NUM").MustInt(20)
	deprecatedSettingDB(rootCfg, "ui", "EXPLORE_PAGING_NUM")
	SitemapPagingNum = sec.Key("SITEMAP_PAGING_NUM").MustInt(20)
	deprecatedSettingDB(rootCfg, "ui", "SITEMAP_PAGING_NUM")
	IssuePagingNum = sec.Key("ISSUE_PAGING_NUM").MustInt(20)
	deprecatedSettingDB(rootCfg, "ui", "ISSUE_PAGING_NUM")
	RepoSearchPagingNum = sec.Key("REPO_SEARCH_PAGING_NUM").MustInt(20)
	deprecatedSettingDB(rootCfg, "ui", "REPO_SEARCH_PAGING_NUM")
	MembersPagingNum = sec.Key("MEMBERS_PAGING_NUM").MustInt(20)
	deprecatedSettingDB(rootCfg, "ui", "MEMBERS_PAGING_NUM")
	FeedMaxCommitNum = sec.Key("FEED_MAX_COMMIT_NUM").MustInt(5)
	deprecatedSettingDB(rootCfg, "ui", "FEED_MAX_COMMIT_NUM")
	FeedPagingNum = sec.Key("FEED_PAGING_NUM").MustInt(20)
	deprecatedSettingDB(rootCfg, "ui", "FEED_PAGING_NUM")
	PackagesPagingNum = sec.Key("PACKAGES_PAGING_NUM").MustInt(20)
	deprecatedSettingDB(rootCfg, "ui", "PACKAGES_PAGING_NUM")
	CodeCommentLines = sec.Key("CODE_COMMENT_LINES").MustInt(4)
	deprecatedSettingDB(rootCfg, "ui", "CODE_COMMENT_LINES")
	ShowUserEmail = sec.Key("SHOW_USER_EMAIL").MustBool(true)
	deprecatedSettingDB(rootCfg, "ui", "SHOW_USER_EMAIL")
	SearchRepoDescription = sec.Key("SEARCH_REPO_DESCRIPTION").MustBool(true)
	deprecatedSettingDB(rootCfg, "ui", "SEARCH_REPO_DESCRIPTION")
	// OnlyShowRelevantRepos=false is important for many private/enterprise instances,
	// because many private repositories do not have "description/topic", users just want to search by their names.
	OnlyShowRelevantRepos = sec.Key("ONLY_SHOW_RELEVANT_REPOS").MustBool(false)
	deprecatedSettingDB(rootCfg, "ui", "ONLY_SHOW_RELEVANT_REPOS")
	ExploreDefaultSort = sec.Key("EXPLORE_PAGING_DEFAULT_SORT").MustString("recentupdate")
	deprecatedSettingDB(rootCfg, "ui", "EXPLORE_PAGING_DEFAULT_SORT")
}
