package webauthn

import (
	"github.com/duo-labs/webauthn/webauthn"

	"code.gitea.io/gitea/modules/setting"
)

//WebAuthn represents the global WebAuthn instance
var WebAuthn *webauthn.WebAuthn

//Init initializes the WebAuthn instance from the config.
func Init() {
	WebAuthn = &webauthn.WebAuthn{
		Config: &webauthn.Config{
			RPDisplayName: setting.AppName,
			RPID:          setting.Domain,
			RPOrigin:      setting.AppURL,
		},
	}
}
