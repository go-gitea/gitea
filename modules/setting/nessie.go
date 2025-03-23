package setting

import (
	"code.gitea.io/gitea/modules/log"
)
	
var Nessie = struct {
	Enabled       bool
	APIURL        string
	DefaultBranch string
	AuthToken     string
}{
	Enabled:       true,
	APIURL:        "http://localhost:19120",
	DefaultBranch: "main",
	AuthToken:     "",
}

func loadNessieFrom(rootCfg ConfigProvider) {
	if err := rootCfg.Section("nessie").MapTo(&Nessie); err != nil {
		log.Fatal("Failed to map Nessie settings: %v", err)
	}
}