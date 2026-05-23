// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/setting/config"
)

// WebBannerType fields are directly used in templates,
// do remember to update the template if you change the fields
type WebBannerType struct {
	DisplayEnabled bool
	ContentMessage string
	StartTimeUnix  int64
	EndTimeUnix    int64
}

func (b WebBannerType) ShouldDisplay() bool {
	if !b.DisplayEnabled || b.ContentMessage == "" {
		return false
	}
	now := time.Now().Unix()
	if b.StartTimeUnix > 0 && now < b.StartTimeUnix {
		return false
	}
	if b.EndTimeUnix > 0 && now > b.EndTimeUnix {
		return false
	}
	return true
}

type MaintenanceModeType struct {
	AdminWebAccessOnly bool
	StartTimeUnix      int64
	EndTimeUnix        int64
}

func (m MaintenanceModeType) IsActive() bool {
	if !m.AdminWebAccessOnly {
		return false
	}
	now := time.Now().Unix()
	if m.StartTimeUnix > 0 && now < m.StartTimeUnix {
		return false
	}
	if m.EndTimeUnix > 0 && now > m.EndTimeUnix {
		return false
	}
	return true
}

type InstanceStruct struct {
	WebBanner       *config.Option[WebBannerType]
	MaintenanceMode *config.Option[MaintenanceModeType]
}
