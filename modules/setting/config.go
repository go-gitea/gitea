// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"context"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting/config"
)

type PictureStruct struct {
	DisableGravatar       *config.Value[bool]
	EnableFederatedAvatar *config.Value[bool]
}

type OpenWithEditorApp struct {
	DisplayName string
	OpenURL     string
}

type OpenWithEditorAppsType []OpenWithEditorApp

func (t OpenWithEditorAppsType) ToTextareaString() string {
	ret := ""
	for _, app := range t {
		ret += app.DisplayName + " = " + app.OpenURL + "\n"
	}
	return ret
}

func DefaultOpenWithEditorApps() OpenWithEditorAppsType {
	return OpenWithEditorAppsType{
		{
			DisplayName: "VS Code",
			OpenURL:     "vscode://vscode.git/clone?url={url}",
		},
		{
			DisplayName: "VSCodium",
			OpenURL:     "vscodium://vscode.git/clone?url={url}",
		},
		{
			DisplayName: "Intellij IDEA",
			OpenURL:     "jetbrains://idea/checkout/git?idea.required.plugins.id=Git4Idea&checkout.repo={url}",
		},
	}
}

type RepositoryStruct struct {
	OpenWithEditorApps *config.Value[OpenWithEditorAppsType]
}

type UIStruct struct {
	ExplorePagingNum          *config.Value[int]
	SitemapPagingNum          *config.Value[int]
	IssuePagingNum            *config.Value[int]
	RepoSearchPagingNum       *config.Value[int]
	MembersPagingNum          *config.Value[int]
	FeedMaxCommitNum          *config.Value[int]
	FeedPagingNum             *config.Value[int]
	PackagesPagingNum         *config.Value[int]
	GraphMaxCommitNum         *config.Value[int]
	CodeCommentLines          *config.Value[int]
	ReactionMaxUserNum        *config.Value[int]
	MaxDisplayFileSize        *config.Value[int64]
	ShowUserEmail             *config.Value[bool]
	DefaultShowFullName       *config.Value[bool]
	DefaultTheme              *config.Value[string]
	Themes                    *config.Value[[]string]
	Reactions                 *config.Value[[]string]
	ReactionsLookup           container.Set[string]
	CustomEmojis              *config.Value[[]string]
	CustomEmojisMap           map[string]string
	SearchRepoDescription     *config.Value[bool]
	OnlyShowRelevantRepos     *config.Value[bool]
	ExploreDefaultSort        *config.Value[string]
	PreferredTimestampTense   *config.Value[string]
	AmbiguousUnicodeDetection *config.Value[bool]
}

func (u *UIStruct) ToStruct(ctx context.Context) UIForm {
	var themes, reactions, customEmojis string
	for _, v := range u.Themes.Value(ctx) {
		themes += v + ","
	}
	themes = strings.TrimSuffix(themes, ",")
	for _, v := range u.Reactions.Value(ctx) {
		reactions += v + ","
	}
	reactions = strings.TrimSuffix(reactions, ",")
	for _, v := range u.CustomEmojis.Value(ctx) {
		customEmojis += v + ","
	}
	customEmojis = strings.TrimSuffix(customEmojis, ",")
	return UIForm{
		ExplorePagingNum:          u.ExplorePagingNum.Value(ctx),
		SitemapPagingNum:          u.SitemapPagingNum.Value(ctx),
		IssuePagingNum:            u.IssuePagingNum.Value(ctx),
		RepoSearchPagingNum:       u.RepoSearchPagingNum.Value(ctx),
		MembersPagingNum:          u.MembersPagingNum.Value(ctx),
		FeedMaxCommitNum:          u.FeedMaxCommitNum.Value(ctx),
		FeedPagingNum:             u.FeedPagingNum.Value(ctx),
		PackagesPagingNum:         u.PackagesPagingNum.Value(ctx),
		GraphMaxCommitNum:         u.GraphMaxCommitNum.Value(ctx),
		CodeCommentLines:          u.CodeCommentLines.Value(ctx),
		ReactionMaxUserNum:        u.ReactionMaxUserNum.Value(ctx),
		MaxDisplayFileSize:        u.MaxDisplayFileSize.Value(ctx),
		ShowUserEmail:             u.ShowUserEmail.Value(ctx),
		DefaultShowFullName:       u.DefaultShowFullName.Value(ctx),
		DefaultTheme:              u.DefaultTheme.Value(ctx),
		Themes:                    themes,
		Reactions:                 reactions,
		CustomEmojis:              customEmojis,
		SearchRepoDescription:     u.SearchRepoDescription.Value(ctx),
		OnlyShowRelevantRepos:     u.OnlyShowRelevantRepos.Value(ctx),
		ExplorePagingDefaultSort:  u.ExploreDefaultSort.Value(ctx),
		PreferredTimestampTense:   u.PreferredTimestampTense.Value(ctx),
		AmbiguousUnicodeDetection: u.AmbiguousUnicodeDetection.Value(ctx),
	}
}

type UIForm struct {
	ExplorePagingNum          int
	SitemapPagingNum          int
	IssuePagingNum            int
	RepoSearchPagingNum       int
	MembersPagingNum          int
	FeedMaxCommitNum          int
	FeedPagingNum             int
	PackagesPagingNum         int
	GraphMaxCommitNum         int
	CodeCommentLines          int
	ReactionMaxUserNum        int
	MaxDisplayFileSize        int64
	ShowUserEmail             bool
	DefaultShowFullName       bool
	DefaultTheme              string
	Themes                    string
	Reactions                 string
	CustomEmojis              string
	SearchRepoDescription     bool
	OnlyShowRelevantRepos     bool
	ExplorePagingDefaultSort  string
	PreferredTimestampTense   string
	AmbiguousUnicodeDetection bool
}

type ConfigStruct struct {
	Picture    *PictureStruct
	Repository *RepositoryStruct
	UI         *UIStruct
}

var (
	defaultConfig     *ConfigStruct
	defaultConfigOnce sync.Once
)

func initDefaultConfig() {
	config.SetCfgSecKeyGetter(&cfgSecKeyGetter{})
	defaultConfig = &ConfigStruct{
		Picture: &PictureStruct{
			DisableGravatar:       config.ValueJSON[bool]("picture.disable_gravatar").WithFileConfig(config.CfgSecKey{Sec: "picture", Key: "DISABLE_GRAVATAR"}),
			EnableFederatedAvatar: config.ValueJSON[bool]("picture.enable_federated_avatar").WithFileConfig(config.CfgSecKey{Sec: "picture", Key: "ENABLE_FEDERATED_AVATAR"}),
		},
		Repository: &RepositoryStruct{
			OpenWithEditorApps: config.ValueJSON[OpenWithEditorAppsType]("repository.open-with.editor-apps"),
		},
		UI: &UIStruct{
			ExplorePagingNum:          config.ValueJSON[int]("ui.explore_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "EXPLORE_PAGING_NUM"}),
			SitemapPagingNum:          config.ValueJSON[int]("ui.sitemap_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "SITEMAP_PAGING_NUM"}),
			IssuePagingNum:            config.ValueJSON[int]("ui.issue_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "ISSUE_PAGING_NUM"}),
			RepoSearchPagingNum:       config.ValueJSON[int]("ui.repo_search_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "REPO_SEARCH_PAGING_NUM"}),
			MembersPagingNum:          config.ValueJSON[int]("ui.members_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "MEMBERS_PAGING_NUM"}),
			FeedMaxCommitNum:          config.ValueJSON[int]("ui.feed_max_commit_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "FEED_MAX_COMMIT_NUM"}),
			FeedPagingNum:             config.ValueJSON[int]("ui.feed_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "FEED_PAGE_NUM"}),
			PackagesPagingNum:         config.ValueJSON[int]("ui.package_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "PACKAGE_PAGING_NUM"}),
			GraphMaxCommitNum:         config.ValueJSON[int]("ui.graph_max_commit_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "GRAPH_MAX_COMMIT_NUM"}),
			CodeCommentLines:          config.ValueJSON[int]("ui.code_comment_lines").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "CODE_COMMENT_LINES"}),
			ReactionMaxUserNum:        config.ValueJSON[int]("ui.reaction_max_user_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "REACTION_MAX_USER_NUM"}),
			MaxDisplayFileSize:        config.ValueJSON[int64]("ui.max_display_file_size").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "MAX_DISPLAY_FILE_SIZE"}),
			ShowUserEmail:             config.ValueJSON[bool]("ui.show_user_email").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "SHOW_USER_EMAIL"}),
			DefaultShowFullName:       config.ValueJSON[bool]("ui.default_show_full_name").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "DEFAULT_SHOW_FULL_NAME"}),
			DefaultTheme:              config.ValueJSON[string]("ui.default_theme").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "DEFAULT_THEME"}),
			Themes:                    config.ValueJSON[[]string]("ui.themes").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "THEMES"}),
			Reactions:                 config.ValueJSON[[]string]("ui.reactions").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "REACTIONS"}),
			CustomEmojis:              config.ValueJSON[[]string]("ui.custom_emojis").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "CUSTOM_EMOJIS"}),
			SearchRepoDescription:     config.ValueJSON[bool]("ui.search_repo_description").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "SEARCH_REPO_DESCRIPTION"}),
			OnlyShowRelevantRepos:     config.ValueJSON[bool]("ui.only_show_relevant_repos").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "ONLY_SHOW_RELEVANT_REPOS"}),
			ExploreDefaultSort:        config.ValueJSON[string]("ui.explore_paging_default_sort").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "EXPLORE_PAGING_DEFAULT_SORT"}),
			PreferredTimestampTense:   config.ValueJSON[string]("ui.preferred_timestamp_tense").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "PREFERRED_TIMESTAMP_TENSE"}),
			AmbiguousUnicodeDetection: config.ValueJSON[bool]("ui.ambiguous_unicode_detection").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "AMBIGUOUS_UNICODE"}),
		},
	}
}

func Config() *ConfigStruct {
	defaultConfigOnce.Do(initDefaultConfig)
	ctx := context.Background()
	defaultConfig.UI.ReactionsLookup = make(container.Set[string])
	for _, reaction := range defaultConfig.UI.Reactions.Value(ctx) {
		defaultConfig.UI.ReactionsLookup.Add(reaction)
	}
	defaultConfig.UI.CustomEmojisMap = make(map[string]string)
	for _, emoji := range defaultConfig.UI.CustomEmojis.Value(ctx) {
		defaultConfig.UI.CustomEmojisMap[emoji] = ":" + emoji + ":"
	}
	return defaultConfig
}

type cfgSecKeyGetter struct{}

func (c cfgSecKeyGetter) GetValue(sec, key string) (v string, has bool) {
	if key == "" {
		return "", false
	}
	cfgSec, err := CfgProvider.GetSection(sec)
	if err != nil {
		log.Error("Unable to get config section: %q", sec)
		return "", false
	}
	cfgKey := ConfigSectionKey(cfgSec, key)
	if cfgKey == nil {
		return "", false
	}
	return cfgKey.Value(), true
}
