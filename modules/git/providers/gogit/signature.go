// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"code.gitea.io/gitea/modules/git/service"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func convertSignature(signature *object.Signature) *service.Signature {
	return &service.Signature{
		Name:  signature.Name,
		Email: signature.Email,
		When:  signature.When,
	}
}
