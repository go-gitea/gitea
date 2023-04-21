// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util/rotatingfilewriter"
)

type Appender interface {
	Record(context.Context, *Event)
	Close() error
	ReleaseReopen() error
}

// NoticeAppender creates an admin notice for every audit event
type NoticeAppender struct{}

func (a *NoticeAppender) Record(ctx context.Context, e *Event) {
	m := fmt.Sprintf("%s\n\nDoer:   %s\nScope:  %s[%v] %s\nTarget: %s[%v] %s", e.Message, e.Doer.FriendlyName, e.Scope.Type, e.Scope.PrimaryKey, e.Scope.FriendlyName, e.Target.Type, e.Target.PrimaryKey, e.Target.FriendlyName)
	if err := system.CreateNotice(ctx, system.NoticeAudit, m); err != nil {
		log.Error("CreateNotice: %v", err)
	}
}

func (a *NoticeAppender) Close() error {
	return nil
}

func (a *NoticeAppender) ReleaseReopen() error {
	return nil
}

// LogAppender writes an info log entry for every audit event
type LogAppender struct{}

func (a *LogAppender) Record(ctx context.Context, e *Event) {
	log.Info(e.Message)
}

func (a *LogAppender) Close() error {
	return nil
}

func (a *LogAppender) ReleaseReopen() error {
	return nil
}

// File writes json object for every audit event
type FileAppender struct {
	rfw *rotatingfilewriter.RotatingFileWriter
}

func NewFileAppender(filename string, opts *rotatingfilewriter.Options) (*FileAppender, error) {
	rfw, err := rotatingfilewriter.Open(filename, opts)
	if err != nil {
		return nil, err
	}

	return &FileAppender{rfw}, nil
}

func (a *FileAppender) Record(ctx context.Context, e *Event) {
	if err := json.NewEncoder(a.rfw).Encode(e); err != nil {
		log.Error("encoding event to file failed: %v", err)
	}
}

func (a *FileAppender) Close() error {
	return a.rfw.Close()
}

func (a *FileAppender) ReleaseReopen() error {
	return a.rfw.ReleaseReopen()
}
