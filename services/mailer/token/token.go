// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package token

import (
	"context"
	crypto_hmac "crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

// A token is a verifiable container describing an action.
//
// A token has a dynamic length depending on the contained data and has the following structure:
// | Token Version | User ID | HMAC | Payload |
//
// The payload is verifiable by the generated HMAC using the user secret. It contains:
// | Timestamp | Action/Handler Type | Action/Handler Data |

const (
	tokenVersion1        byte = 1
	tokenLifetimeInYears int  = 1
)

type HandlerType byte

const (
	UnknownHandlerType HandlerType = iota
	ReplyHandlerType
	UnsubscribeHandlerType
)

var encodingWithoutPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

type ErrToken struct {
	context string
}

func (err *ErrToken) Error() string {
	return "invalid email token: " + err.context
}

func (err *ErrToken) Unwrap() error {
	return util.ErrInvalidArgument
}

// CreateToken creates a token for the action/user tuple
func CreateToken(ht HandlerType, user *user_model.User, data []byte) (string, error) {
	payload, err := util.PackData(
		time.Now().AddDate(tokenLifetimeInYears, 0, 0).Unix(),
		ht,
		data,
	)
	if err != nil {
		return "", err
	}

	packagedData, err := util.PackData(
		user.ID,
		generateHmac([]byte(user.Rands), payload),
		payload,
	)
	if err != nil {
		return "", err
	}

	return encodingWithoutPadding.EncodeToString(append([]byte{tokenVersion1}, packagedData...)), nil
}

// ExtractToken extracts the action/user tuple from the token and verifies the content
func ExtractToken(ctx context.Context, token string) (HandlerType, *user_model.User, []byte, error) {
	data, err := encodingWithoutPadding.DecodeString(token)
	if err != nil {
		return UnknownHandlerType, nil, nil, err
	}

	if len(data) < 1 {
		return UnknownHandlerType, nil, nil, &ErrToken{"no data"}
	}

	if data[0] != tokenVersion1 {
		return UnknownHandlerType, nil, nil, &ErrToken{fmt.Sprintf("unsupported token version: %v", data[0])}
	}

	var userID int64
	var hmac []byte
	var payload []byte
	if err := util.UnpackData(data[1:], &userID, &hmac, &payload); err != nil {
		return UnknownHandlerType, nil, nil, err
	}

	user, err := user_model.GetUserByID(ctx, userID)
	if err != nil {
		return UnknownHandlerType, nil, nil, err
	}

	if !crypto_hmac.Equal(hmac, generateHmac([]byte(user.Rands), payload)) {
		return UnknownHandlerType, nil, nil, &ErrToken{"verification failed"}
	}

	var expiresUnix int64
	var handlerType HandlerType
	var innerPayload []byte
	if err := util.UnpackData(payload, &expiresUnix, &handlerType, &innerPayload); err != nil {
		return UnknownHandlerType, nil, nil, err
	}

	if time.Unix(expiresUnix, 0).Before(time.Now()) {
		return UnknownHandlerType, nil, nil, &ErrToken{"token expired"}
	}

	return handlerType, user, innerPayload, nil
}

// generateHmac creates a trunkated HMAC for the given payload
func generateHmac(secret, payload []byte) []byte {
	mac := crypto_hmac.New(sha256.New, secret)
	mac.Write(payload)
	hmac := mac.Sum(nil)

	return hmac[:10] // RFC2104 recommends not using less then 80 bits
}
