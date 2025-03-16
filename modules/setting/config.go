// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"context"
	"sync"

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
	ExplorePagingNum      *config.Value[int]
	SitemapPagingNum      *config.Value[int]
	IssuePagingNum        *config.Value[int]
	RepoSearchPagingNum   *config.Value[int]
	MembersPagingNum      *config.Value[int]
	FeedMaxCommitNum      *config.Value[int]
	FeedPagingNum         *config.Value[int]
	PackagesPagingNum     *config.Value[int]
	CodeCommentLines      *config.Value[int]
	ShowUserEmail         *config.Value[bool]
	SearchRepoDescription *config.Value[bool]
	OnlyShowRelevantRepos *config.Value[bool]
	ExploreDefaultSort    *config.Value[string]
}

func (u *UIStruct) ToStruct(ctx context.Context) UIForm {
	return UIForm{
		ExplorePagingNum:         u.ExplorePagingNum.Value(ctx),
		SitemapPagingNum:         u.SitemapPagingNum.Value(ctx),
		IssuePagingNum:           u.IssuePagingNum.Value(ctx),
		RepoSearchPagingNum:      u.RepoSearchPagingNum.Value(ctx),
		MembersPagingNum:         u.MembersPagingNum.Value(ctx),
		FeedMaxCommitNum:         u.FeedMaxCommitNum.Value(ctx),
		FeedPagingNum:            u.FeedPagingNum.Value(ctx),
		PackagesPagingNum:        u.PackagesPagingNum.Value(ctx),
		CodeCommentLines:         u.CodeCommentLines.Value(ctx),
		ShowUserEmail:            u.ShowUserEmail.Value(ctx),
		SearchRepoDescription:    u.SearchRepoDescription.Value(ctx),
		OnlyShowRelevantRepos:    u.OnlyShowRelevantRepos.Value(ctx),
		ExplorePagingDefaultSort: u.ExploreDefaultSort.Value(ctx),
		ExplorePagingSortOption:  []string{"recentupdate", "alphabetically", "reverselastlogin", "newest", "oldest"},
	}
}

type UIForm struct {
	ExplorePagingNum         int
	SitemapPagingNum         int
	IssuePagingNum           int
	RepoSearchPagingNum      int
	MembersPagingNum         int
	FeedMaxCommitNum         int
	FeedPagingNum            int
	PackagesPagingNum        int
	CodeCommentLines         int
	ShowUserEmail            bool
	SearchRepoDescription    bool
	OnlyShowRelevantRepos    bool
	ExplorePagingDefaultSort string
	ExplorePagingSortOption  []string
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
			ExplorePagingNum:      config.ValueJSON[int]("ui.explore_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "EXPLORE_PAGING_NUM"}).WithDefault(20),
			SitemapPagingNum:      config.ValueJSON[int]("ui.sitemap_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "SITEMAP_PAGING_NUM"}).WithDefault(20),
			IssuePagingNum:        config.ValueJSON[int]("ui.issue_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "ISSUE_PAGING_NUM"}).WithDefault(20),
			RepoSearchPagingNum:   config.ValueJSON[int]("ui.repo_search_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "REPO_SEARCH_PAGING_NUM"}).WithDefault(20),
			MembersPagingNum:      config.ValueJSON[int]("ui.members_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "MEMBERS_PAGING_NUM"}).WithDefault(20),
			FeedMaxCommitNum:      config.ValueJSON[int]("ui.feed_max_commit_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "FEED_MAX_COMMIT_NUM"}).WithDefault(20),
			FeedPagingNum:         config.ValueJSON[int]("ui.feed_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "FEED_PAGE_NUM"}).WithDefault(20),
			PackagesPagingNum:     config.ValueJSON[int]("ui.packages_paging_num").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "PACKAGES_PAGING_NUM"}).WithDefault(20),
			CodeCommentLines:      config.ValueJSON[int]("ui.code_comment_lines").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "CODE_COMMENT_LINES"}).WithDefault(4),
			ShowUserEmail:         config.ValueJSON[bool]("ui.show_user_email").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "SHOW_USER_EMAIL"}).WithDefault(true),
			SearchRepoDescription: config.ValueJSON[bool]("ui.search_repo_description").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "SEARCH_REPO_DESCRIPTION"}).WithDefault(false),
			OnlyShowRelevantRepos: config.ValueJSON[bool]("ui.only_show_relevant_repos").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "ONLY_SHOW_RELEVANT_REPOS"}).WithDefault(false),
			ExploreDefaultSort:    config.ValueJSON[string]("ui.explore_paging_default_sort").WithFileConfig(config.CfgSecKey{Sec: "ui", Key: "EXPLORE_PAGING_DEFAULT_SORT"}).WithDefault("recentupdate"),
		},
	}
}

func Config() *ConfigStruct {
	defaultConfigOnce.Do(initDefaultConfig)
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
