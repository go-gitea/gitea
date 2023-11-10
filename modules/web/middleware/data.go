// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// ContextDataStore represents a data store
type ContextDataStore interface {
	GetData() ContextData
}

type ContextData map[string]any

func (ds ContextData) GetData() ContextData {
	return ds
}

func (ds ContextData) MergeFrom(other ContextData) ContextData {
	for k, v := range other {
		ds[k] = v
	}
	return ds
}

const ContextDataKeySignedUser = "SignedUser"

type contextDataKeyType struct{}

var contextDataKey contextDataKeyType

func WithContextData(c context.Context) context.Context {
	return context.WithValue(c, contextDataKey, make(ContextData, 10))
}

func GetContextData(c context.Context) ContextData {
	if ds, ok := c.Value(contextDataKey).(ContextData); ok {
		return ds
	}
	return nil
}

func CommonTemplateContextData() ContextData {
	return ContextData{
		"IsLandingPageOrganizations": setting.LandingPageURL == setting.LandingPageOrganizations,

		"ShowRegistrationButton":        setting.Service.ShowRegistrationButton,
		"ShowMilestonesDashboardPage":   setting.Service.ShowMilestonesDashboardPage,
		"ShowFooterVersion":             setting.Other.ShowFooterVersion,
		"DisableDownloadSourceArchives": setting.Repository.DisableDownloadSourceArchives,

		"EnableSwagger":      setting.API.EnableSwagger,
		"EnableOpenIDSignIn": setting.Service.EnableOpenIDSignIn,
		"PageStartTime":      time.Now(),

		"RunModeIsProd": setting.IsProd,
	}
}
