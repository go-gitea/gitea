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
)

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
	if entry.IsDir() {
		return []string{"directory"}
	}

	if entry.IsRegular() {
		fileName := strings.ToLower(path.Base(entry.Name()))
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(fileName), "."))
		return []string{fileName, ext}
	}

	return nil
}

func loadCustomIcon(iconPath string) (string, error) {
	// Try to load the icon from the filesystem
	if _, err := os.Stat(iconPath); err != nil {
		return "", err
	}

	// Read the SVG file
	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		return "", err
	}

	return string(iconData), nil
}

// FileIcon returns a custom icon from a folder or the default octicon for displaying files/directories
func FileIcon(ctx context.Context, entry *git.TreeEntry) template.HTML {
	iconPack, ok := ctx.Value("icon-pack").(string) // TODO: allow user to select an icon pack from a list
	iconPack = "demo"
	ok = true

	if ok && iconPack != "" {
		iconNames := getFileIconNames(entry)

		// Try to load the custom icon
		for _, iconName := range iconNames {
			iconPath := path.Join(setting.AppDataPath, "icons", iconPack, iconName+".svg")
			if icon, err := loadCustomIcon(iconPath); err == nil {
				return template.HTML(icon)
			}
		}
	}

	// If no custom icon was found or an error occurred, return the default icon
	return svg.RenderHTML(getBasicFileIconName(entry))
}
