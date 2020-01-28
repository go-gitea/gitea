// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
)

const (
	// The tests will fail if the waiter function takes less than
	// blockerDelay minus tolerance to complete.
	// Note: these values might require tuning in order to avoid
	// false negatives.
	waiterDelay  = 100 * time.Millisecond
	blockerDelay = 200 * time.Millisecond
	tolerance    = 50 * time.Millisecond // Should be <= (blockerDelay-waiterDelay)/2
)

type waitResult struct {
	Waited time.Duration
	Err    error
}

func TestLockedResource(t *testing.T) {
	defer prepareTestEnv(t)()

	// We need to check whether two goroutines block each other
	// Sadly, there's no way to ensure the second goroutine is
	// waiting other than using a time delay. The longer the delay,
	// the more certain we are the second goroutine is waiting.

	// This check **must** fail as we're not blocking anything
	assert.Error(t, blockTest("no block", func(ctx models.DBContext) error {
		return nil
	}))

	models.AssertNotExistsBean(t, &models.LockedResource{LockType: "test-1", LockKey: 1})

	// Test with creation (i.e. new lock type)
	assert.NoError(t, blockTest("block-new", func(ctx models.DBContext) error {
		_, err := models.GetLockedResourceCtx(ctx, "block-test-1", 1)
		return err
	}))

	// Test without creation (i.e. lock type already exists)
	assert.NoError(t, blockTest("block-existing", func(ctx models.DBContext) error {
		_, err := models.GetLockedResourceCtx(ctx, "block-test-1", 1)
		return err
	}))

	// Test with temporary record
	assert.NoError(t, blockTest("block-temp", func(ctx models.DBContext) error {
		return models.TemporarilyLockResourceKeyCtx(ctx, "temp-1", 1)
	}))
}

func blockTest(name string, f func(ctx models.DBContext) error) error {
	cb := make(chan waitResult)
	cw := make(chan waitResult)
	ref := time.Now()

	go func() {
		cb <- blockTestFunc(name, true, ref, f)
	}()
	go func() {
		cw <- blockTestFunc(name, false, ref, f)
	}()

	resb := <-cb
	resw := <-cw
	if resb.Err != nil {
		return resb.Err
	}
	if resw.Err != nil {
		return resw.Err
	}

	if resw.Waited < blockerDelay-tolerance {
		return fmt.Errorf("Waiter not blocked on %s; wait: %d ms, expected > %d ms",
			name, resw.Waited.Milliseconds(), (blockerDelay - tolerance).Milliseconds())
	}

	return nil
}

func blockTestFunc(name string, blocker bool, ref time.Time, f func(ctx models.DBContext) error) (wr waitResult) {
	if blocker {
		name = fmt.Sprintf("blocker [%s]", name)
	} else {
		name = fmt.Sprintf("waiter [%s]", name)
	}
	err := models.WithTx(func(ctx models.DBContext) error {
		log.Trace("Entering %s @%d", name, time.Since(ref).Milliseconds())
		if !blocker {
			log.Trace("Waiting on %s @%d", name, time.Since(ref).Milliseconds())
			time.Sleep(waiterDelay)
			log.Trace("Wait finished on %s @%d", name, time.Since(ref).Milliseconds())
		}
		if err := f(ctx); err != nil {
			return err
		}
		if blocker {
			log.Trace("Waiting on %s @%d", name, time.Since(ref).Milliseconds())
			time.Sleep(blockerDelay)
			log.Trace("Wait finished on %s @%d", name, time.Since(ref).Milliseconds())
		} else {
			wr.Waited = time.Since(ref)
		}
		log.Trace("Finishing %s @%d", name, time.Since(ref).Milliseconds())
		return nil
	})
	if err != nil {
		wr.Err = fmt.Errorf("error in %s: %v", name, err)
	}
	return
}
