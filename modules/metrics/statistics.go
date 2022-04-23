// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"strconv"
	"sync"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/setting"
)

var statisticsLock sync.Mutex

func GetStatistic(estimate bool, statisticsTTL time.Duration, metrics bool) models.Statistic {
	if statisticsTTL > 0 {
		c := cache.GetCache()
		if c != nil {
			statisticsLock.Lock()
			defer statisticsLock.Unlock()
			cacheKey := "models/statistic.Statistic." + strconv.FormatBool(estimate) + strconv.FormatBool(metrics)

			if stats, ok := c.Get(cacheKey).(*models.Statistic); ok {
				return *stats
			}

			stats := models.GetStatistic(estimate, metrics)
			c.Put(cacheKey, &stats, setting.DurationToCacheTTL(statisticsTTL))
			return stats
		}
	}

	return models.GetStatistic(estimate, metrics)
}
