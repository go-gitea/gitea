package merlin

import (
	"k8s.io/apimachinery/pkg/util/sets"

	"code.gitea.io/gitea/modules/setting"
)

var validLicense sets.String

func initConfig() {
	license := setting.CfgProvider.Section("merlin").Key("LICENSE").Strings(",")
	validLicense = sets.NewString()
	for _, v := range license {
		validLicense.Insert(v)
	}
}
