// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"gitea.dev/modules/cache"
	"gitea.dev/modules/json"
	"gitea.dev/modules/timeutil"
)

const (
	requiredAdditionalInfoFailureWarningCacheKey = "oauth2.required.additional.info.failure.warning"
	requiredAdditionalInfoFailureWarningTTL      = 3600
	requiredAdditionalInfoFailureWarningThrottle = 60
)

type RequiredAdditionalInfoFailureWarning struct {
	SourceName     string             `json:"sourceName"`
	LastFailedUnix timeutil.TimeStamp `json:"lastFailedUnix"`
}

func GetRequiredAdditionalInfoFailureWarning(c cache.StringCache) *RequiredAdditionalInfoFailureWarning {
	if c == nil {
		return nil
	}

	rawWarning, ok := c.Get(requiredAdditionalInfoFailureWarningCacheKey)
	if !ok || rawWarning == "" {
		return nil
	}

	warning := &RequiredAdditionalInfoFailureWarning{}
	if err := json.Unmarshal([]byte(rawWarning), warning); err != nil {
		_ = c.Delete(requiredAdditionalInfoFailureWarningCacheKey)
		return nil
	}
	if warning.SourceName == "" || warning.LastFailedUnix.IsZero() {
		_ = c.Delete(requiredAdditionalInfoFailureWarningCacheKey)
		return nil
	}

	return warning
}

func SetRequiredAdditionalInfoFetchFailureWarning(c cache.StringCache, sourceName string) {
	if c == nil {
		return
	}
	if sourceName == "" {
		sourceName = "OAuth2"
	}

	now := timeutil.TimeStampNow()
	current := GetRequiredAdditionalInfoFailureWarning(c)
	if current != nil && current.SourceName == sourceName && now-current.LastFailedUnix < requiredAdditionalInfoFailureWarningThrottle {
		return
	}

	rawWarning, err := json.Marshal(&RequiredAdditionalInfoFailureWarning{
		SourceName:     sourceName,
		LastFailedUnix: now,
	})
	if err != nil {
		return
	}
	if err := c.Put(requiredAdditionalInfoFailureWarningCacheKey, string(rawWarning), requiredAdditionalInfoFailureWarningTTL); err != nil {
		return
	}
}

func ClearRequiredAdditionalInfoFetchFailureWarning(c cache.StringCache) {
	if c == nil {
		return
	}
	_ = c.Delete(requiredAdditionalInfoFailureWarningCacheKey)
}
