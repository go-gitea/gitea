// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
	ap "gitea.com/Ta180m/activitypub"
)

// GetItemByType instantiates a new ForgeFed object if the type matches
// otherwise it defaults to existing activitypub package typer function.
func GetItemByType(typ ap.ActivityVocabularyType) (ap.Item, error) {
	if typ == CommitType {
		return CommitNew(), nil
	} else if typ == BranchType {
		return BranchNew(), nil
	} else if typ == RepositoryType {
		return RepositoryNew(""), nil
	} else if typ == PushType {
		return PushNew(), nil
	} else if typ == TicketType {
		return TicketNew(), nil
	}
	return ap.GetItemByType(typ)
}
