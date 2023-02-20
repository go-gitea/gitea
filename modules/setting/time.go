// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"
)

var (
	// Time settings
	TimeFormat string
	// UILocation is the location on the UI, so that we can display the time on UI.
	DefaultUILocation = time.Local
)

func loadTimeFrom(rootCfg ConfigProvider) {
	timeFormatKey := rootCfg.Section("time").Key("FORMAT").MustString("")
	if timeFormatKey != "" {
		TimeFormat = map[string]string{
			"ANSIC":       time.ANSIC,
			"UnixDate":    time.UnixDate,
			"RubyDate":    time.RubyDate,
			"RFC822":      time.RFC822,
			"RFC822Z":     time.RFC822Z,
			"RFC850":      time.RFC850,
			"RFC1123":     time.RFC1123,
			"RFC1123Z":    time.RFC1123Z,
			"RFC3339":     time.RFC3339,
			"RFC3339Nano": time.RFC3339Nano,
			"Kitchen":     time.Kitchen,
			"Stamp":       time.Stamp,
			"StampMilli":  time.StampMilli,
			"StampMicro":  time.StampMicro,
			"StampNano":   time.StampNano,
		}[timeFormatKey]
		// When the TimeFormatKey does not exist in the previous map e.g.'2006-01-02 15:04:05'
		if len(TimeFormat) == 0 {
			TimeFormat = timeFormatKey
			TestTimeFormat, _ := time.Parse(TimeFormat, TimeFormat)
			if TestTimeFormat.Format(time.RFC3339) != "2006-01-02T15:04:05Z" {
				log.Warn("Provided TimeFormat: %s does not create a fully specified date and time.", TimeFormat)
				log.Warn("In order to display dates and times correctly please check your time format has 2006, 01, 02, 15, 04 and 05")
			}
			log.Trace("Custom TimeFormat: %s", TimeFormat)
		}
	}

	zone := rootCfg.Section("time").Key("DEFAULT_UI_LOCATION").String()
	if zone != "" {
		var err error
		DefaultUILocation, err = time.LoadLocation(zone)
		if err != nil {
			log.Fatal("Load time zone failed: %v", err)
		} else {
			log.Info("Default UI Location is %v", zone)
		}
	}
	if DefaultUILocation == nil {
		DefaultUILocation = time.Local
	}
}
