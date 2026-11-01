// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import "context"

type StorageMutation string

const (
	StoragePush  StorageMutation = "push"
	StorageFetch StorageMutation = "fetch"
	StorageClone StorageMutation = "clone"
)

type (
	StorageMutationRunner func(context.Context, RepositoryFacade, []string) error
	StorageMutationGuard  func(context.Context, RepositoryFacade, StorageMutation, StorageMutationRunner) error
)

// GuardStorageMutation is installed by the storage policy service before workers start.
var GuardStorageMutation StorageMutationGuard = func(ctx context.Context, repo RepositoryFacade, _ StorageMutation, run StorageMutationRunner) error {
	return run(ctx, repo, nil)
}
