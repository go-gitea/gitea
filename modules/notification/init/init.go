package init

import (
	// Import all needed receivers
	_ "code.gitea.io/gitea/modules/notification/action"
	_ "code.gitea.io/gitea/modules/notification/indexer"
	_ "code.gitea.io/gitea/modules/notification/mail"
	_ "code.gitea.io/gitea/modules/notification/ui"
	_ "code.gitea.io/gitea/modules/notification/webhook"
)
