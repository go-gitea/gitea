package fileicon

import (
	"os"
	"path"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type fileIconFolderBackend struct {
	theme string
}

func (f *fileIconFolderBackend) GetIcon(iconName string) (string, error) {
	iconPath := path.Join(setting.AppDataPath, "icons", f.theme, iconName+".svg")

	log.Debug("Loading custom icon from %s", iconPath)

	// Try to load the icon from the filesystem
	if _, err := os.Stat(iconPath); err != nil {
		return "", err
	}

	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		return "", err
	}

	return string(iconData), nil
}
