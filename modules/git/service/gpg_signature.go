// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// GPGSignature represents a git commit signature part.
type GPGSignature struct {
	Signature string
	Payload   string //TODO check if can be reconstruct from the rest of commit information to not have duplicate data
}
