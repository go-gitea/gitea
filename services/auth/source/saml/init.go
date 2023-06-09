package saml

import (
	"sync"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/log"
)

var samlRWMutex = sync.RWMutex{}

func Init() error {
	loginSources, _ := auth.GetActiveSAMLProviderLoginSources()
	for _, source := range loginSources {
		samlSource, ok := source.Cfg.(*Source)
		if !ok {
			continue
		}
		err := samlSource.RegisterSource()
		if err != nil {
			log.Error("Unable to register source: %s due to Error: %v.", source.Name, err)
		}
	}
	return nil
}
