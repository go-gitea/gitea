/*
Copyright (c) Huawei Technologies Co., Ltd. 2024. All rights reserved
*/

// Package configHooks for check license

package configHooks

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

type FileSizeLimit struct {
	 Name string
	 Content string
}

func (c FileSizeLimit) GetHookName() string {
	return c.Name
}

func (c FileSizeLimit) GetHookContent() string {
	if setting.CommonMaxFileSize > 0 {
		c.Content = fmt.Sprintf(`
max_size=%d

while read oldrev newrev _; do
  if [[ "$oldrev" == "0000000000000000000000000000000000000000" ]]; then
    files=$(git ls-tree --name-only ${newrev})
    for file in $files; do
      size=$(git cat-file -s ${newrev}:${file})
      if [[ ${size} -gt ${max_size} ]]; then
		    echo "The size of each file should be within $((max_size / 1048576))MB."
		    exit 1
	    fi
	  done
  else
    changes=$(git rev-list ${oldrev}..${newrev})

    for commit in ${changes}; do
      files=$(git diff-tree --no-commit-id --name-only -r ${commit})

      for file in $files; do
        size=$(git cat-file -s ${commit}:${file})
        if [[ ${size} -gt ${max_size} ]]; then
          echo "The size of each file should be within $((max_size / 1048576))MB."
          exit 1
        fi
      done
    done
  fi
done
`, setting.CommonMaxFileSize)
	}
	return c.Content
}


