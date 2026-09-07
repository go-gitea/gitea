// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/lfs"
)

const copiedContentAssociationAttempts = 3

var (
	// ErrCopiedContentRepoNotExist is returned when the destination repository
	// cannot be resolved inside the association transaction.
	ErrCopiedContentRepoNotExist = errors.New("copied content destination repository does not exist")
	// ErrCopiedContentSizeConflict is returned when an existing or requested
	// association disagrees about an OID's size.
	ErrCopiedContentSizeConflict = errors.New("copied content LFS association size conflict")
	// ErrCopiedContentRetryExhausted is returned after uniqueness/serialization
	// errors exhaust the short transaction retries.
	ErrCopiedContentRetryExhausted = errors.New("copied content association transaction retries exhausted")
)

// CopiedContentLFSPointer is a destination LFS pointer that must be associated
// with the target repository.
type CopiedContentLFSPointer struct {
	Oid  string
	Size int64
}

// AssociateCopiedContentLFS inserts missing destination LFS associations for
// the supplied pointers in one transaction. Existing associations are verified
// and left in place. Unique/serialization conflicts retry the whole transaction.
func AssociateCopiedContentLFS(ctx context.Context, repoID int64, pointers []CopiedContentLFSPointer) error {
	deduped, err := dedupeCopiedContentPointers(pointers)
	if err != nil {
		return err
	}
	if len(deduped) == 0 {
		return nil
	}

	var lastErr error
	for range copiedContentAssociationAttempts {
		lastErr = db.WithTx(ctx, func(ctx context.Context) error {
			return associateCopiedContentLFSTx(ctx, repoID, deduped)
		})
		if lastErr == nil {
			return nil
		}
		if errors.Is(lastErr, ErrCopiedContentSizeConflict) || errors.Is(lastErr, ErrCopiedContentRepoNotExist) {
			return lastErr
		}
		if !isUniqueOrSerializationError(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("%w: %v", ErrCopiedContentRetryExhausted, lastErr)
}

func associateCopiedContentLFSTx(ctx context.Context, repoID int64, pointers []CopiedContentLFSPointer) error {
	if _, err := repo_model.GetRepositoryByID(ctx, repoID); err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return ErrCopiedContentRepoNotExist
		}
		return err
	}

	missing := make([]*LFSMetaObject, 0, len(pointers))
	for _, pointer := range pointers {
		existing, err := GetLFSMetaObjectByOid(ctx, repoID, pointer.Oid)
		if err != nil {
			if !errors.Is(err, ErrLFSObjectNotExist) {
				return err
			}
			missing = append(missing, &LFSMetaObject{
				Pointer:      lfs.Pointer{Oid: pointer.Oid, Size: pointer.Size},
				RepositoryID: repoID,
			})
			continue
		}
		if existing.Size != pointer.Size {
			return ErrCopiedContentSizeConflict
		}
	}

	for _, meta := range missing {
		if err := db.Insert(ctx, meta); err != nil {
			return err
		}
	}
	return nil
}

func dedupeCopiedContentPointers(pointers []CopiedContentLFSPointer) ([]CopiedContentLFSPointer, error) {
	byOID := make(map[string]CopiedContentLFSPointer, len(pointers))
	ordered := make([]CopiedContentLFSPointer, 0, len(pointers))
	for _, pointer := range pointers {
		if existing, ok := byOID[pointer.Oid]; ok {
			if existing.Size != pointer.Size {
				return nil, ErrCopiedContentSizeConflict
			}
			continue
		}
		byOID[pointer.Oid] = pointer
		ordered = append(ordered, pointer)
	}
	return ordered, nil
}

func isUniqueOrSerializationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unique constraint"),
		strings.Contains(msg, "unique_violation"),
		strings.Contains(msg, "duplicate key"),
		strings.Contains(msg, "duplicate entry"),
		strings.Contains(msg, "violates unique"),
		strings.Contains(msg, "could not serialize"),
		strings.Contains(msg, "serialization failure"),
		strings.Contains(msg, "deadlock"),
		strings.Contains(msg, "database is locked"),
		strings.Contains(msg, "database table is locked"),
		strings.Contains(msg, "sqlite_busy"):
		return true
	default:
		return false
	}
}
