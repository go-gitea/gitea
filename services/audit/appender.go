// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/log"
)

type Appender interface {
	Record(context.Context, *Event)
	Close()
}

// NoticeAppender creates an admin notice for every audit event
type NoticeAppender struct{}

func (a *NoticeAppender) Record(ctx context.Context, e *Event) {
	m := fmt.Sprintf("%s\n\nDoer:   %s\nScope:  %s[%v] %s\nTarget: %s[%v] %s", e.Message, e.Doer.FriendlyName, e.Scope.Type, e.Scope.PrimaryKey, e.Scope.FriendlyName, e.Target.Type, e.Target.PrimaryKey, e.Target.FriendlyName)
	if err := system.CreateNotice(ctx, system.NoticeAudit, m); err != nil {
		log.Error("CreateNotice: %v", err)
	}
}

func (a *NoticeAppender) Close() {
}

// LogAppender writes an info log entry for every audit event
type LogAppender struct{}

func (a *LogAppender) Record(ctx context.Context, e *Event) {
	log.Info(e.Message)
}

func (a *LogAppender) Close() {
}
