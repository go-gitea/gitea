package fileicon

import (
	"context"
	"html/template"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	lru "github.com/hashicorp/golang-lru/v2"
)

var fileIconCache *lru.Cache[string, string]

func init() {
	var err error
	fileIconCache, err = lru.New[string, string](1000)
	if err != nil {
		log.Fatal("Failed to create file icon cache: %v", err)
	}
}

func getBasicFileIconName(entry *git.TreeEntry) string {
	switch {
	case entry.IsLink():
		te, err := entry.FollowLink()
		if err != nil {
			log.Debug(err.Error())
			return "octicon-file-symlink-file"
		}
		if te.IsDir() {
			return "octicon-file-directory-symlink"
		}
		return "octicon-file-symlink-file"
	case entry.IsDir():
		return "octicon-file-directory-fill"
	case entry.IsSubModule():
		return "octicon-file-submodule"
	}

	return "octicon-file"
}

func getFileIconNames(entry *git.TreeEntry) []string {
	fileName := strings.ToLower(path.Base(entry.Name()))

	if entry.IsDir() {
		return []string{"folder_" + fileName, "folder"}
	}

	if entry.IsRegular() {
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(fileName), "."))
		return []string{"file_" + fileName, "file_" + ext, "file"}
	}

	return nil
}

func loadCustomIcon(iconPath string) (string, error) {
	log.Info("Loading custom icon from %s", iconPath)

	if icon, ok := fileIconCache.Get(iconPath); ok {
		return icon, nil
	}

	// Try to load the icon from the filesystem
	if _, err := os.Stat(iconPath); err != nil {
		return "", err
	}

	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		return "", err
	}

	fileIconCache.Add(iconPath, string(iconData))

	return string(iconData), nil
}

// FileIcon returns a custom icon from a folder or the default octicon for displaying files/directories
func FileIcon(ctx context.Context, entry *git.TreeEntry) template.HTML {
	iconTheme := setting.UI.FileIconTheme
	if iconTheme != "" {
		iconNames := getFileIconNames(entry)

		// Try to load the custom icon
		for _, iconName := range iconNames {
			iconPath := path.Join(setting.AppDataPath, "icons", iconTheme, iconName+".svg")
			if icon, err := loadCustomIcon(iconPath); err == nil {
				return svg.RenderHTMLFromString(icon)
			}
		}
	}

	// If no custom icon was found or an error occurred, return the default icon
	return svg.RenderHTML(getBasicFileIconName(entry))
}
