// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import "context"

type baseDummy struct{}

var _ baseQueue = (*baseDummy)(nil)

func newBaseDummy(cfg *BaseConfig, unique bool) (baseQueue, error) {
	return &baseDummy{}, nil
}

func (q *baseDummy) PushItem(ctx context.Context, data []byte) error {
	return nil
}

func (q *baseDummy) PopItem(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (q *baseDummy) Len(ctx context.Context) (int, error) {
	return 0, nil
}

func (q *baseDummy) HasItem(ctx context.Context, data []byte) (bool, error) {
	return false, nil
}

func (q *baseDummy) Close() error {
	return nil
}

func (q *baseDummy) RemoveAll(ctx context.Context) error {
	return nil
}
