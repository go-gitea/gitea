// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"sync"

	"code.gitea.io/gitea/modules/git/objectpool"
	"code.gitea.io/gitea/modules/log"
)

type catFileObjectPoolProvider struct {
	repoPath  string
	mu        sync.Mutex
	pool      catFileBatchCloser
	poolInUse bool
}

var _ objectpool.Provider = (*catFileObjectPoolProvider)(nil)

// NewObjectPoolProvider creates a "batch object provider (CatFileBatch)" for the given repository path to retrieve object info and content efficiently.
// The CatFileBatch and the readers create by it should only be used in the same goroutine.
func NewObjectPoolProvider(repoPath string) objectpool.Provider {
	return &catFileObjectPoolProvider{repoPath: repoPath}
}

func (p *catFileObjectPoolProvider) GetObjectPool(ctx context.Context) (objectpool.ObjectPool, func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var err error
	if p.pool == nil {
		p.pool, err = newObjectPool(ctx, p.repoPath)
		if err != nil {
			p.pool = nil // otherwise it is "interface(nil)" and will cause wrong logic
			return nil, nil, err
		}
	}

	if !p.poolInUse {
		p.poolInUse = true
		return p.pool, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.poolInUse = false
		}, nil
	}

	log.Debug("Opening temporary cat file batch for: %s", p.repoPath)
	tempBatch, err := newObjectPool(ctx, p.repoPath)
	if err != nil {
		return nil, nil, err
	}
	return tempBatch, tempBatch.Close, nil
}

type catFileBatchCloser interface {
	objectpool.ObjectPool
	Close()
}

func newObjectPool(ctx context.Context, repoPath string) (catFileBatchCloser, error) {
	if DefaultFeatures().SupportCatFileBatchCommand {
		return newCatFileBatchCommand(ctx, repoPath)
	}
	return newCatFileBatchLegacy(ctx, repoPath)
}

func (p *catFileObjectPoolProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pool != nil {
		p.pool.Close()
		p.pool = nil
		p.poolInUse = false
	}
}
