// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build stress

package models

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// StressIssueNoDupIndex Performs a stress test of the INSERT ... SELECT function of database for inserting issues
func StressIssueNoDupIndex(t *testing.T, useTransactions bool, initialIssueFill int, maxTestDuration int, threadCount int) {
	assert.NoError(t, PrepareTestDatabase())

	// Defaults
	const defInitialIssueFill = 1000 // issues inserted prior to stress test
	const defMaxTestDuration = 60    // seconds
	const defThreadCount = 8         // max simultaneous threads

	if initialIssueFill == 0 {
		initialIssueFill = defInitialIssueFill
	}
	if maxTestDuration == 0 {
		maxTestDuration = defMaxTestDuration
	}
	if threadCount == 0 {
		threadCount = defThreadCount
	}

	var err error
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	doer := AssertExistsAndLoadBean(t, &User{ID: repo.OwnerID}).(*User)

	// Pre-load
	for i := 1; i < initialIssueFill; i++ {
		issue := &Issue{
			RepoID:   repo.ID,
			PosterID: repo.OwnerID,
			Index:    int64(i + 5000), // Avoid clashing with other tests
			Title:    fmt.Sprintf("NoDup initial %d", i),
		}
		_, err = x.Insert(issue)
		assert.NoError(t, err)
	}

	fmt.Printf("TestIssueNoDupIndex(): %d rows created\n", initialIssueFill)

	until := time.Now().Add(time.Second * time.Duration(maxTestDuration))

	var hasErrors int32
	var wg sync.WaitGroup

	f := func(thread int) {
		defer wg.Done()
		sess := x.NewSession()
		defer sess.Close()
		i := 1
		for {
			if time.Now().After(until) || atomic.LoadInt32(&hasErrors) != 0 {
				return
			}
			issue := &Issue{
				RepoID:         repo.ID,
				PosterID:       repo.OwnerID,
				Title:          fmt.Sprintf("NoDup stress %d, %d", thread, i),
				OriginalAuthor: "TestIssueNoDupIndex()",
				Priority:       thread, // For statistics
			}
			if useTransactions {
				if err = sess.Begin(); err != nil {
					break
				}
			}
			if err = newIssue(sess, doer, NewIssueOptions{
				Repo:  repo,
				Issue: issue,
			}); err != nil {
				break
			}
			if useTransactions {
				if err = sess.Commit(); err != nil {
					break
				}
			}
			i++
		}
		if useTransactions {
			_ = sess.Rollback()
		}
		atomic.StoreInt32(&hasErrors, 1)
		t.Logf("newIssue(): %+v", err)
	}

	for i := 1; i <= threadCount; i++ {
		go f(i)
		wg.Add(1)
	}

	fmt.Printf("TestIssueNoDupIndex(): %d threads created\n", threadCount)

	wg.Wait()

	for i := 1; i <= threadCount; i++ {
		total, err := x.Table("issue").
			Where("original_author = ?", "TestIssueNoDupIndex()").
			And("priority = ?", i).
			Count()
		assert.NoError(t, err, "Failed counting generated issues count")
		fmt.Printf("TestIssueNoDupIndex(): rows created in thread #%d: %d\n", i, total)
	}

	assert.Equal(t, int32(0), hasErrors, "Synchronization errors detected.")
}
