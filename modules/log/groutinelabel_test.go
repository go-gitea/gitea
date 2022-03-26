// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"context"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getGoroutineLabels(t *testing.T) {
	pprof.Do(context.Background(), pprof.Labels(), func(ctx context.Context) {
		currentLabels := getGoroutineLabels()
		pprof.ForLabels(ctx, func(key, value string) bool {
			assert.EqualValues(t, value, currentLabels[key])
			return true
		})

		pprof.Do(ctx, pprof.Labels("Test_getGoroutineLabels", "Test_getGoroutineLabels_child1"), func(ctx context.Context) {
			currentLabels := getGoroutineLabels()
			pprof.ForLabels(ctx, func(key, value string) bool {
				assert.EqualValues(t, value, currentLabels[key])
				return true
			})
			if assert.NotNil(t, currentLabels) {
				assert.EqualValues(t, "Test_getGoroutineLabels_child1", currentLabels["Test_getGoroutineLabels"])
			}
		})
	})
}
