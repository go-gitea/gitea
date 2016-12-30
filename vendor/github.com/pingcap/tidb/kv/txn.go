// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kv

import (
	"math"
	"math/rand"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
)

// RunInNewTxn will run the f in a new transaction environment.
func RunInNewTxn(store Storage, retryable bool, f func(txn Transaction) error) error {
	for i := 0; i < maxRetryCnt; i++ {
		txn, err := store.Begin()
		if err != nil {
			log.Errorf("[kv] RunInNewTxn error - %v", err)
			return errors.Trace(err)
		}

		err = f(txn)
		if retryable && IsRetryableError(err) {
			log.Warnf("[kv] Retry txn %v", txn)
			txn.Rollback()
			continue
		}
		if err != nil {
			txn.Rollback()
			return errors.Trace(err)
		}

		err = txn.Commit()
		if retryable && IsRetryableError(err) {
			log.Warnf("[kv] Retry txn %v", txn)
			txn.Rollback()
			BackOff(i)
			continue
		}
		if err != nil {
			return errors.Trace(err)
		}
		break
	}

	return nil
}

var (
	// Max retry count in RunInNewTxn
	maxRetryCnt = 100
	// retryBackOffBase is the initial duration, in microsecond, a failed transaction stays dormancy before it retries
	retryBackOffBase = 1
	// retryBackOffCap is the max amount of duration, in microsecond, a failed transaction stays dormancy before it retries
	retryBackOffCap = 100
)

// BackOff Implements exponential backoff with full jitter.
// Returns real back off time in microsecond.
// See: http://www.awsarchitectureblog.com/2015/03/backoff.html.
func BackOff(attempts int) int {
	upper := int(math.Min(float64(retryBackOffCap), float64(retryBackOffBase)*math.Pow(2.0, float64(attempts))))
	sleep := time.Duration(rand.Intn(upper)) * time.Millisecond
	time.Sleep(sleep)
	return int(sleep)
}
