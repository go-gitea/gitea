// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
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
	GitGuideRemoteName *config.Value[string]
}

type ServiceStruct struct {
	DefaultKeepEmailPrivate                 *config.Value[bool]
	DefaultAllowCreateOrganization          *config.Value[bool]
	DefaultUserIsRestricted                 *config.Value[bool]
	EnableTimeTracking                      *config.Value[bool]
	DefaultEnableTimeTracking               *config.Value[bool]
	DefaultEnableDependencies               *config.Value[bool]
	AllowCrossRepositoryDependencies        *config.Value[bool]
	DefaultAllowOnlyContributorsToTrackTime *config.Value[bool]
	EnableUserHeatmap                       *config.Value[bool]
	AutoWatchNewRepos                       *config.Value[bool]
	AutoWatchOnChanges                      *config.Value[bool]
	DefaultOrgMemberVisible                 *config.Value[bool]
}

type ConfigStruct struct {
	Picture    *PictureStruct
	Repository *RepositoryStruct
	Service    *ServiceStruct
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
			GitGuideRemoteName: config.ValueJSON[string]("repository.git-guide-remote-name").WithDefault("origin"),
		},
		Service: &ServiceStruct{
			DefaultKeepEmailPrivate:                 config.ValueJSON[bool]("service.default_keep_email_private").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "DEFAULT_KEEP_EMAIL_PRIVATE"}).WithDefault(false),
			DefaultAllowCreateOrganization:          config.ValueJSON[bool]("service.default_allow_create_organization").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "DEFAULT_ALLOW_CREATE_ORGANIZATION"}).WithDefault(true),
			DefaultUserIsRestricted:                 config.ValueJSON[bool]("service.default_user_is_restricted").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "DEFAULT_USER_IS_RESTRICTED"}).WithDefault(false),
			EnableTimeTracking:                      config.ValueJSON[bool]("service.enable_time_tracking").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "ENABLE_TIMETRACKING"}).WithDefault(true),
			DefaultEnableTimeTracking:               config.ValueJSON[bool]("service.default_enable_time_tracking").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "DEFAULT_ENABLE_TIMETRACKING"}).WithDefault(true),
			DefaultEnableDependencies:               config.ValueJSON[bool]("service.default_enable_dependencies").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "DEFAULT_ENABLE_DEPENDENCIES"}).WithDefault(true),
			AllowCrossRepositoryDependencies:        config.ValueJSON[bool]("service.allow_cross_repository_dependencies").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "ALLOW_CROSS_REPOSITORY_DEPENDENCIES"}).WithDefault(true),
			DefaultAllowOnlyContributorsToTrackTime: config.ValueJSON[bool]("service.default_allow_only_contributors_to_track_time").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "DEFAULT_ALLOW_ONLY_CONTRIBUTORS_TO_TRACK_TIME"}).WithDefault(true),
			EnableUserHeatmap:                       config.ValueJSON[bool]("service.enable_user_heatmap").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "ENABLE_USER_HEATMAP"}).WithDefault(true),
			AutoWatchNewRepos:                       config.ValueJSON[bool]("service.auto_watch_new_repos").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "AUTO_WATCH_NEW_REPOS"}).WithDefault(true),
			AutoWatchOnChanges:                      config.ValueJSON[bool]("service.auto_watch_on_changes").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "AUTO_WATCH_ON_CHANGES"}).WithDefault(false),
			DefaultOrgMemberVisible:                 config.ValueJSON[bool]("service.default_org_member_visible").WithFileConfig(config.CfgSecKey{Sec: "service", Key: "DEFAULT_ORG_MEMBER_VISIBLE"}).WithDefault(false),
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
