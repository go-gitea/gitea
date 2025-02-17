// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package payload

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

const replyPayloadVersion1 byte = 1

type payloadReferenceType byte

const (
	payloadReferenceIssue payloadReferenceType = iota
	payloadReferenceComment
	payloadReferenceNewIssue
	payloadReferenceNewPullRequest
)

type ReferenceRepositoryActionType int64

const (
	ReferenceRepositoryActionTypeNewIssue ReferenceRepositoryActionType = iota
	ReferenceRepositoryActionTypeNewPullRequest
)

type ReferenceRepository struct {
	RepositoryID int64
	ActionType   ReferenceRepositoryActionType
}

// CreateReferencePayload creates data which GetReferenceFromPayload resolves to the reference again.
func CreateReferencePayload(reference any) ([]byte, error) {
	var refType payloadReferenceType
	var refID int64

	switch r := reference.(type) {
	case *issues_model.Issue:
		refType = payloadReferenceIssue
		refID = r.ID
	case *issues_model.Comment:
		refType = payloadReferenceComment
		refID = r.ID
	case *ReferenceRepository:
		switch r.ActionType {
		case ReferenceRepositoryActionTypeNewIssue:
			refType = payloadReferenceNewIssue
			refID = r.RepositoryID
		case ReferenceRepositoryActionTypeNewPullRequest:
			refType = payloadReferenceNewPullRequest
			refID = r.RepositoryID
		default:
			return nil, util.NewInvalidArgumentErrorf("unsupported repository reference action type: %d", r.ActionType)
		}
	default:
		return nil, util.NewInvalidArgumentErrorf("unsupported reference type: %T", r)
	}

	payload, err := util.PackData(refType, refID)
	if err != nil {
		return nil, err
	}

	return append([]byte{replyPayloadVersion1}, payload...), nil
}

// GetReferenceFromPayload resolves the reference from the payload
func GetReferenceFromPayload(ctx context.Context, payload []byte) (any, error) {
	if len(payload) < 1 {
		return nil, util.NewInvalidArgumentErrorf("payload to small")
	}

	if payload[0] != replyPayloadVersion1 {
		return nil, util.NewInvalidArgumentErrorf("unsupported payload version")
	}

	var ref payloadReferenceType
	var id int64
	if err := util.UnpackData(payload[1:], &ref, &id); err != nil {
		return nil, err
	}

	switch ref {
	case payloadReferenceIssue:
		return issues_model.GetIssueByID(ctx, id)
	case payloadReferenceComment:
		return issues_model.GetCommentByID(ctx, id)
	case payloadReferenceNewIssue:
		return repo_model.GetRepositoryByID(ctx, id)
	case payloadReferenceNewPullRequest:
		return repo_model.GetRepositoryByID(ctx, id)
	default:
		return nil, util.NewInvalidArgumentErrorf("unsupported reference type: %T", ref)
	}
}

func GetRandsFromPayload(ctx context.Context, doer *user_model.User, payload []byte) []byte {
	if len(payload) < 1 {
		return []byte{}
	}

	if payload[0] != replyPayloadVersion1 {
		return []byte{}
	}

	var ref payloadReferenceType
	var id int64
	if err := util.UnpackData(payload[1:], &ref, &id); err != nil {
		return []byte{}
	}

	switch ref {
	case payloadReferenceIssue:
		return []byte(doer.Rands)
	case payloadReferenceComment:
		return []byte(doer.Rands)
	case payloadReferenceNewIssue:
		rands, _ := user_model.GetRandsForRepository(ctx, doer.ID, id, user_model.RepositoryRandsTypeNewIssue)
		return []byte(rands)
	case payloadReferenceNewPullRequest:
		return []byte{}
	default:
		return []byte{}
	}
}
