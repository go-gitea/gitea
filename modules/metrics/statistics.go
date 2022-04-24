// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"strconv"
	"sync"
	"time"

	"code.gitea.io/gitea/models"
)

var (
	statisticsLock        sync.Mutex
	statisticsMap         = map[string]*models.Statistic{}
	statisticsWorkingChan = map[string]chan struct{}{}
)

func GetStatistic(estimate bool, statisticsTTL time.Duration, metrics bool) <-chan *models.Statistic {
	cacheKey := "models/statistic.Statistic." + strconv.FormatBool(estimate) + strconv.FormatBool(metrics)

	statisticsLock.Lock() // CAREFUL: no defer!
	ourChan := make(chan *models.Statistic, 1)

	// Check for a cached statistic
	if statisticsTTL > 0 {
		if stats, ok := statisticsMap[cacheKey]; ok && stats.Time.Add(statisticsTTL).After(time.Now()) {
			// Found a valid cached statistic for these params, so unlock and send this down the channel
			statisticsLock.Unlock() // Unlock from line 24

			ourChan <- stats
			close(ourChan)
			return ourChan
		}
	}

	// We need to calculate a statistic - however, we should only do this one at a time (NOTE: we are still within the lock)
	//
	// So check if we have a worker already and get a marker channel
	workingChan, ok := statisticsWorkingChan[cacheKey]

	if !ok {
		// we need to make our own worker... (NOTE: we are still within the lock)

		// create a marker channel which will be closed when our worker is finished
		// and assign it to the working map.
		workingChan = make(chan struct{})
		statisticsWorkingChan[cacheKey] = workingChan

		// Create the working go-routine
		go func() {
			stats := models.GetStatistic(estimate, metrics)

			// cache the result, remove this worker and inform anyone waiting we are done
			statisticsLock.Lock() // Lock within goroutine
			statisticsMap[cacheKey] = &stats
			delete(statisticsWorkingChan, cacheKey)
			close(workingChan)
			statisticsLock.Unlock() // Unlock within goroutine
		}()
	}

	statisticsLock.Unlock() // Unlock from line 24

	// Create our goroutine for the channel waiting for the statistics to be generated
	go func() {
		<-workingChan // Wait for the worker to finish

		// Now lock and get the last stats completed
		statisticsLock.Lock()
		stats := statisticsMap[cacheKey]
		statisticsLock.Unlock()

		ourChan <- stats
		close(ourChan)
	}()

	return ourChan
}
