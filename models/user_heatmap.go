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
func GetUserHeatmapDataByUser(user *User) (hdata []*UserHeatmapData, err error) {

	var groupBy string
	switch {
	case setting.UseSQLite3:
		groupBy = "strftime('%Y-%m-%d', created_unix, 'unixepoch')"
	case setting.UseTiDB:
	case setting.UseMySQL:
		groupBy = "DATE_FORMAT(FROM_UNIXTIME(created_unix), '%Y%m%d')"
	case setting.UsePostgreSQL:
		groupBy = "date_trunc('day', created_unix)"
	case setting.UseMSSQL:
		groupBy = "dateadd(DAY,0, datediff(day,0, dateadd(s, created_unix, '19700101')))"
	}

	err = x.Select("created_unix as timestamp, count(user_id) as contributions").
		Table("action").
		Where("user_id = ?", user.ID).
		And("created_unix > ?", (util.TimeStampNow() - 31536000)).
		GroupBy(groupBy).
		OrderBy("created_unix").
		Find(&hdata)
	return
}
