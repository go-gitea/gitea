package repository

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/options"
)

func GetGitAttributes(name string) ([]byte, error) {
	data, err := options.GitAttributes(name)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("GetRepoInitFile[%s]: %w", name, err)
	}

	return data, nil
}
