// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"math"
	"time"

	"code.gitea.io/gitea/modules/setting/config"
	"code.gitea.io/gitea/modules/util"
)

// WebBannerType fields are directly used in templates,
// do remember to update the template if you change the fields
type WebBannerType struct {
	DisplayEnabled  bool
	ContentMessage  string
	BackgroundColor string
	StartTimeUnix   int64
	EndTimeUnix     int64
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

const defaultWebBannerBackgroundColor = "#ddf4ff"

func mixWebBannerColor(baseColor, mixColor string, mixWeight float64) string {
	if mixWeight <= 0 {
		return baseColor
	}
	if mixWeight >= 1 {
		return mixColor
	}

	baseR, baseG, baseB := util.HexToRBGColor(baseColor)
	mixR, mixG, mixB := util.HexToRBGColor(mixColor)

	r := uint8(math.Round(baseR*(1-mixWeight) + mixR*mixWeight))
	g := uint8(math.Round(baseG*(1-mixWeight) + mixG*mixWeight))
	b := uint8(math.Round(baseB*(1-mixWeight) + mixB*mixWeight))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func (b WebBannerType) NormalizedBackgroundColor() string {
	normalized, err := util.NormalizeColor(b.BackgroundColor)
	if err != nil {
		return ""
	}
	return normalized
}

func (b WebBannerType) BackgroundColorForForm() string {
	if backgroundColor := b.NormalizedBackgroundColor(); backgroundColor != "" {
		return backgroundColor
	}
	return defaultWebBannerBackgroundColor
}

func (b WebBannerType) DefaultBackgroundColor() string {
	return defaultWebBannerBackgroundColor
}

func (b WebBannerType) TextColor() string {
	backgroundColor := b.NormalizedBackgroundColor()
	if backgroundColor == "" {
		return ""
	}
	return util.ContrastColor(backgroundColor)
}

func (b WebBannerType) BorderColor() string {
	backgroundColor := b.NormalizedBackgroundColor()
	if backgroundColor == "" {
		return ""
	}
	return mixWebBannerColor(backgroundColor, util.ContrastColor(backgroundColor), 0.18)
}

func (b WebBannerType) FormValue() WebBannerType {
	b.BackgroundColor = b.BackgroundColorForForm()
	return b
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
