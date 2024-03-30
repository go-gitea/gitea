/*
Copyright (c) Huawei Technologies Co., Ltd. 2024. All rights reserved
*/

// Package configHooks for check license

package configHooks

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

type CheckLicense struct {
	 Name string
	 Content string
}

func (c CheckLicense) GetHookName() string {
	return c.Name
}

func (c CheckLicense) GetHookContent() string {
	license := setting.CfgProvider.Section("merlin").Key("LICENSE").Strings(",")
	if len(license) > 0 {
		shellLicense := strings.Join(license, " ")
		c.Content = fmt.Sprintf(`
valid_licenses=(%s)

while read oldrev newrev _; do
	files=$(git diff --name-only $oldrev $newrev)
  if echo "$files" | grep -q "README.md"; then
		readme_content=$(git show $newrev:README.md)
		license=$(echo "$readme_content" | grep -oP "license=\[\K[^]]+")
		if [[ " ${valid_licenses[@]} " =~ " ${license} " ]]; then
				echo "License field is valid. Proceeding with the push."
		else
				echo "Sorry, your push was rejected during YAML metadata verification:"
				echo " - Error: "license" must be one of (${valid_licenses[@]})"
				exit 1
		fi
  fi
done
`, shellLicense)
	}
	return c.Content
}


