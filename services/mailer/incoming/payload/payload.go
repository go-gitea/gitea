// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package payload

import (
	"context"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/util"
)

const replyPayloadVersion1 byte = 1

type payloadReferenceType byte

const (
	payloadReferenceIssue payloadReferenceType = iota
	payloadReferenceComment
)

// CreateReferencePayload creates data which GetReferenceFromPayload resolves to the reference again.
func CreateReferencePayload(reference interface{}) ([]byte, error) {
	var refType payloadReferenceType
	var refID int64

	switch r := reference.(type) {
	case *issues_model.Issue:
		refType = payloadReferenceIssue
		refID = r.ID
	case *issues_model.Comment:
		refType = payloadReferenceComment
		refID = r.ID
	default:
		return nil, fmt.Errorf("unsupported reference type")
	}

	payload, err := util.PackData(refType, refID)
	if err != nil {
		return nil, err
	}

	return append([]byte{replyPayloadVersion1}, payload...), nil
}

// GetReferenceFromPayload resolves the reference from the payload
func GetReferenceFromPayload(ctx context.Context, payload []byte) (interface{}, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("payload to small")
	}

	if payload[0] != replyPayloadVersion1 {
		return nil, fmt.Errorf("unsupported payload version")
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
	}

	return nil, fmt.Errorf("unsupported reference type")
}
