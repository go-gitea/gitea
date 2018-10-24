// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package models

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     util.TimeStamp `json:"timestamp"`
	Contributions int64          `json:"contributions"`
}

// GetUserHeatmapDataByUser returns an array of UserHeatmapData
func GetUserHeatmapDataByUser(user *User) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)
	var groupBy string
	switch {
	case setting.UseSQLite3:
		groupBy = "strftime('%s', strftime('%Y-%m-%d', created_unix, 'unixepoch'))"
	case setting.UseMySQL:
		groupBy = "UNIX_TIMESTAMP(DATE_FORMAT(FROM_UNIXTIME(created_unix), '%Y%m%d'))"
	case setting.UsePostgreSQL:
		groupBy = "extract(epoch from date_trunc('day', to_timestamp(created_unix)))"
	case setting.UseMSSQL:
		groupBy = "dateadd(DAY,0, datediff(day,0, dateadd(s, created_unix, '19700101')))"
	}

	err := x.Select(groupBy+" as timestamp, count(user_id) as contributions").
		Table("action").
		Where("user_id = ?", user.ID).
		And("created_unix > ?", (util.TimeStampNow() - 31536000)).
		GroupBy("timestamp").
		OrderBy("timestamp").
		Find(&hdata)
	return hdata, err
}
