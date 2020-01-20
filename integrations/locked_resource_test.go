// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"testing"
	"time"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

const (
	// Note: these values might require tuning
	before = 500 * time.Millisecond
	after = 1000 * time.Millisecond
	tolerance = 200 * time.Millisecond
)

type waitResult struct {
	Waited	time.Duration
	Err		error
}

func TestLockedResource(t *testing.T) {
	defer prepareTestEnv(t)()

	// We need to check whether two goroutines block each other
	// Sadly, there's no way to ensure the second goroutine is
	// waiting other than using a time delay. The longer the delay,
	// the more certain we are the second goroutine is waiting.

	// This check **must** fail as we're not blocking anything
	assert.Error(t, blockTest("no block", func(ctx models.DBContext) (func() error, error){
		return func() error{
			return nil
		}, nil
	}))

	models.AssertNotExistsBean(t, &models.LockedResource{LockType: "test-1", LockKey: 1})

	// Test with creation (i.e. new lock type)
	assert.NoError(t, blockTest("block-new", func(ctx models.DBContext) (func() error, error){
		_, err := models.GetLockedResourceCtx(ctx, "block-test-1", 1)
		return func() error{
			return nil
		}, err
	}))

	// Test without creation (i.e. lock type already exists)
	assert.NoError(t, blockTest("block-existing", func(ctx models.DBContext) (func() error, error){
		_, err := models.GetLockedResourceCtx(ctx, "block-test-1", 1)
		return func() error{
			return nil
		}, err
	}))

	// Test with temporary record
	assert.NoError(t, blockTest("block-temp", func(ctx models.DBContext) (func() error, error){
		return models.TempLockResourceCtx(ctx, "temp-1", 1)
	}))
}

func blockTest(name string, f func(ctx models.DBContext) (func() error, error)) error {
	cb := make(chan waitResult)
	cw := make(chan waitResult)
	ref := time.Now()

	go func() {
		cb <- blockTestFunc(name, true, ref, f)
	}()
	go func() {
		cw <- blockTestFunc(name, false, ref, f)
	}()

	resb := <- cb
	resw := <- cw
	if resb.Err != nil {
		return resb.Err
	}
	if resw.Err != nil {
		return resw.Err
	}

	if resw.Waited < after - tolerance {
		return fmt.Errorf("Waiter not blocked on %s; wait: %d ms, expected > %d ms",
			name, resw.Waited.Milliseconds(), (after - tolerance).Milliseconds())
	}

	return nil
}

func blockTestFunc(name string, blocker bool, ref time.Time, f func(ctx models.DBContext) (func() error, error)) (wr waitResult) {
	if blocker {
		name = fmt.Sprintf("blocker [%s]", name)
	} else {
		name = fmt.Sprintf("waiter [%s]", name)
	}
	err := models.WithTx(func(ctx models.DBContext) error {
		fmt.Printf("Entering %s @%d\n", name, time.Now().Sub(ref).Milliseconds())
		if !blocker {
			fmt.Printf("Waiting on %s @%d\n", name, time.Now().Sub(ref).Milliseconds())
			time.Sleep(before)
			fmt.Printf("Wait finished on %s @%d\n", name, time.Now().Sub(ref).Milliseconds())
		}
		releaseLock, err := f(ctx)
		if err != nil {
			return err
		}
		if blocker {
			fmt.Printf("Waiting on %s @%d\n", name, time.Now().Sub(ref).Milliseconds())
			time.Sleep(after)
			fmt.Printf("Wait finished on %s @%d\n", name, time.Now().Sub(ref).Milliseconds())
		} else {
			wr.Waited = time.Now().Sub(ref)
		}
		fmt.Printf("Finishing %s @%d\n", name, time.Now().Sub(ref).Milliseconds())
		return releaseLock()
	})
	if err != nil {
		wr.Err = fmt.Errorf("error in %s: %v", name, err)
	}
	return
}
