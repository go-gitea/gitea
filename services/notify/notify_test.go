// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notify

import (
	"context"
	"errors"
	"testing"
	"time"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

type panicNotifier struct {
	NullNotifier
	panicValue any
	called     bool
}

func (n *panicNotifier) NewIssue(context.Context, *issues_model.Issue, []*user_model.User) {
	n.called = true
	panic(n.panicValue)
}

func TestNotificationPanicRecovery(t *testing.T) {
	notifier := &panicNotifier{}
	defer test.MockVariableValue(&notifiers, []Notifier{notifier})()

	for _, panicValue := range []any{"notification panic", errors.New("notification error"), nil} {
		notifier.panicValue = panicValue
		notifier.called = false
		assert.NotPanics(t, func() { NewIssue(t.Context(), nil, nil) })
		assert.True(t, notifier.called)
	}

	checker, cleanup := test.NewLogChecker(log.DEFAULT)
	defer cleanup()
	checker.Filter("PANIC while notifying: notification panic", "notify_test.go", "notify.go").StopMark("notification recovery complete")
	notifier.panicValue = "notification panic"
	assert.NotPanics(t, func() { NewIssue(t.Context(), nil, nil) })
	log.Info("notification recovery complete")
	filtered, stopped := checker.Check(time.Second)
	assert.True(t, stopped)
	assert.Equal(t, []bool{true, true, true}, filtered)

	assert.NotPanics(t, func() { CreateIssueComment(t.Context(), nil, nil, nil, nil, nil) })
	assert.NotPanics(t, func() { NewPullRequest(t.Context(), nil, nil) })
}
