package models

import (
	"code.gitea.io/gitea/modules/util"
)

type UserHeatmapData struct {
	Timestamp     util.TimeStamp
	Contributions int64
}

func GetUserHeatmapDataByUser(user *User) (*map[util.TimeStamp]int64, error) {
	var hdata []UserHeatmapData
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

	// Bring our heatmap in the format needed by cal-heatmap
	fullHeatmap := make(map[util.TimeStamp]int64)
	for _, h := range hdata {
		fullHeatmap[h.Timestamp] = h.Contributions
	}

	return &fullHeatmap, err
}
