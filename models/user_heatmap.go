package models

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type UserHeatmapData struct {
	Timestamp     util.TimeStamp `json:"timestamp"`
	Contributions int64          `json:"contributions"`
}

func GetUserHeatmapDataByUser(user *User) (hdata []*UserHeatmapData, err error) {
	//var hdata []UserHeatmapData

	sec := setting.Cfg.Section("database")
	dbtype := sec.Key("DB_TYPE").String()
	// Sqlite doesn't has the "DATE_FORMAT" function, so we need a special case for that
	if dbtype == "sqlite3" {
		err := x.Select("created_unix as timestamp, count(user_id) as contributions").
			Table("action").
			Where("user_id = ?", user.ID).
			And("created_unix > ?", (util.TimeStampNow() - 31536000)).
			GroupBy("strftime('%Y-%m-%d', created_unix, 'unixepoch')").
			OrderBy("created_unix").
			Find(&hdata)
		if err != nil {
			return nil, err
		}
	} else {
		err := x.Select("created_unix as timestamp, count(user_id) as contributions").
			Table("action").
			Where("user_id = ?", user.ID).
			And("created_unix > ?", (util.TimeStampNow() - 31536000)).
			GroupBy("DATE_FORMAT(FROM_UNIXTIME(created_unix), '%Y%m%d')").
			OrderBy("created_unix").
			Find(&hdata)
		if err != nil {
			return nil, err
		}
	}

	// Bring our heatmap in the format needed by cal-heatmap
	/*fullHeatmap := make(map[util.TimeStamp]int64)
	for _, h := range hdata {
		fullHeatmap[h.Timestamp] = h.Contributions
	}

	return &fullHeatmap, nil*/
	return
}
