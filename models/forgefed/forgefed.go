// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
	ap "github.com/go-ap/activitypub"
)

const ForgeFedNamespaceURI = "https://forgefed.org/ns"

// GetItemByType instantiates a new ForgeFed object if the type matches
// otherwise it defaults to existing activitypub package typer function.
func GetItemByType(typ ap.ActivityVocabularyType) (ap.Item, error) {
	switch typ  {
	case CommitType:
		return CommitNew(), nil
	case BranchType:
		return BranchNew(), nil
	case RepositoryType:
		return RepositoryNew(""), nil
	case PushType:
		return PushNew(), nil
	case TicketType:
		return TicketNew(), nil
	}
	return ap.GetItemByType(typ)
}
