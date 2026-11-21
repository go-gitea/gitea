// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"slices"
	"sync"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"

	"xorm.io/builder"
)

type tasksVersionScope struct {
	ownerID, repoID int64
}

var tasksVersionWakes = struct {
	sync.Mutex
	chans   map[tasksVersionScope]chan struct{}
	waiters int
}{chans: map[tasksVersionScope]chan struct{}{}}

var (
	tasksVersionWatchInterval = time.Second
	tasksVersionWatchOnce     sync.Once
)

// TasksVersionWakeChannel must be read before the version to avoid missing a wake.
func TasksVersionWakeChannel(ownerID, repoID int64) <-chan struct{} {
	tasksVersionWakes.Lock()
	defer tasksVersionWakes.Unlock()
	scope := tasksVersionScope{ownerID, repoID}
	ch, ok := tasksVersionWakes.chans[scope]
	if !ok {
		ch = make(chan struct{})
		tasksVersionWakes.chans[scope] = ch
	}
	return ch
}

// WatchTasksVersion marks a waiter until released, so the process's watcher wakes waiters of scopes whose version was
// changed by another process.
func WatchTasksVersion() (release func()) {
	tasksVersionWatchOnce.Do(func() { go watchTasksVersions() })
	tasksVersionWakes.Lock()
	tasksVersionWakes.waiters++
	tasksVersionWakes.Unlock()
	return func() {
		tasksVersionWakes.Lock()
		tasksVersionWakes.waiters--
		tasksVersionWakes.Unlock()
	}
}

func watchTasksVersions() {
	ctx := graceful.GetManager().ShutdownContext()
	ticker := time.NewTicker(tasksVersionWatchInterval)
	defer ticker.Stop()
	lastVersions := map[tasksVersionScope]int64{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		tasksVersionWakes.Lock()
		if tasksVersionWakes.waiters == 0 {
			tasksVersionWakes.Unlock()
			continue
		}
		scopes := make([]tasksVersionScope, 0, len(tasksVersionWakes.chans))
		for scope := range tasksVersionWakes.chans {
			scopes = append(scopes, scope)
		}
		tasksVersionWakes.Unlock()

		versions, err := getTasksVersions(ctx, scopes)
		if err != nil {
			log.Error("watchTasksVersions: %v", err)
			continue
		}
		var changed []tasksVersionScope
		for _, scope := range scopes {
			// a scope without a previous version may have changed before it was first read
			if last, ok := lastVersions[scope]; !ok || last != versions[scope] {
				changed = append(changed, scope)
			}
			lastVersions[scope] = versions[scope]
		}

		tasksVersionWakes.Lock()
		for _, scope := range changed {
			if ch, ok := tasksVersionWakes.chans[scope]; ok {
				close(ch)
				delete(tasksVersionWakes.chans, scope)
			}
		}
		tasksVersionWakes.Unlock()
	}
}

func getTasksVersions(ctx context.Context, scopes []tasksVersionScope) (map[tasksVersionScope]int64, error) {
	versions := make(map[tasksVersionScope]int64, len(scopes))
	for chunk := range slices.Chunk(scopes, 500) {
		cond := builder.NewCond()
		for _, scope := range chunk {
			cond = cond.Or(builder.Eq{"owner_id": scope.ownerID, "repo_id": scope.repoID})
		}
		var rows []*ActionTasksVersion
		if err := db.GetEngine(ctx).Where(cond).Cols("owner_id", "repo_id", "version").Find(&rows); err != nil {
			return nil, err
		}
		for _, row := range rows {
			versions[tasksVersionScope{row.OwnerID, row.RepoID}] = row.Version
		}
	}
	return versions, nil
}

// AfterUpdate runs once the outermost transaction committed.
func (v *ActionTasksVersion) AfterUpdate() {
	tasksVersionWakes.Lock()
	defer tasksVersionWakes.Unlock()
	scope := tasksVersionScope{v.OwnerID, v.RepoID}
	if ch, ok := tasksVersionWakes.chans[scope]; ok {
		close(ch)
		delete(tasksVersionWakes.chans, scope)
	}
}
