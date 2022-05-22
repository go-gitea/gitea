// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/process"
)

var (
	statisticsLock sync.Mutex

	shortStatistic *models.Statistic
	fullStatistic  *models.Statistic

	shortStatisticWorkingChan chan struct{}
	fullStatisticWorkingChan  chan struct{}
)

func GetStatistic(statisticsTTL time.Duration, full bool) <-chan *models.Statistic {
	statisticsLock.Lock() // CAREFUL: no defer!
	ourChan := make(chan *models.Statistic, 1)

	// Check for a cached statistic
	if statisticsTTL > 0 {
		stats := fullStatistic
		if !full && shortStatistic != nil {
			stats = shortStatistic
		}
		if stats != nil && stats.Time.Add(statisticsTTL).After(time.Now()) {
			// Found a valid cached statistic for these params, so unlock and send this down the channel
			statisticsLock.Unlock() // Unlock from above

			ourChan <- stats
			close(ourChan)
			return ourChan
		}
	}

	// We need to calculate a statistic - however, we should only do this one at a time (NOTE: we are still within the lock)
	//
	// So check if we have a worker already and get a marker channel
	var workingChan chan struct{}
	if full {
		workingChan = fullStatisticWorkingChan
	} else {
		workingChan = shortStatisticWorkingChan
	}

	if workingChan == nil {
		// we need to make our own worker... (NOTE: we are still within the lock)

		// create a marker channel which will be closed when our worker is finished
		// and assign it to the working map.
		workingChan = make(chan struct{})
		if full {
			fullStatisticWorkingChan = workingChan
		} else {
			shortStatisticWorkingChan = workingChan
		}

		// Create the working go-routine
		go func() {
			ctx, _, finished := process.GetManager().AddContext(db.DefaultContext, fmt.Sprintf("Statistics: Full: %t", full))
			defer finished()
			stats := models.GetStatistic(ctx, full)
			statsPtr := &stats
			select {
			case <-ctx.Done():
				// The above stats likely have been cancelled part way through generation and should be ignored
				statsPtr = nil
			default:
			}

			// cache the result, remove this worker and inform anyone waiting we are done
			statisticsLock.Lock() // Lock within goroutine
			if statsPtr != nil {
				shortStatistic = statsPtr
				if full {
					fullStatistic = statsPtr
				}
			}
			if full {
				fullStatisticWorkingChan = nil
			} else {
				shortStatisticWorkingChan = nil
			}
			close(workingChan)
			statisticsLock.Unlock() // Unlock within goroutine
		}()
	}

	statisticsLock.Unlock() // Unlock from above

	// Create our goroutine for the channel waiting for the statistics to be generated
	go func() {
		<-workingChan // Wait for the worker to finish

		// Now lock and get the last stats completed
		statisticsLock.Lock()
		stats := fullStatistic
		if !full {
			stats = shortStatistic
		}
		statisticsLock.Unlock()

		ourChan <- stats
		close(ourChan)
	}()

	return ourChan
}
