package fileicon

import (
	"context"
	"html/template"
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

// getFileIconNames returns a list of possible icon names for a file or directory
// Folder named `sub-folder` =>
//   - `folder_sub-folder“ (. will be replaced with _)
//   - `folder`
//
// File named `.gitignore` =>
//   - `file__gitignore` (. will be replaced with _)
//   - `file_`
//
// File named `README.md` =>
//   - `file_readme_md`
//   - `file_md`
func getFileIconNames(entry *git.TreeEntry) []string {
	fileName := strings.ReplaceAll(strings.ToLower(path.Base(entry.Name())), ".", "_")

	if entry.IsDir() {
		return []string{"folder_" + fileName, "folder"}
	}

	if entry.IsRegular() {
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(entry.Name()), "."))
		return []string{"file_" + fileName, "file_" + ext, "file"}
	}

	return nil
}

type fileIconBackend interface {
	GetIcon(string) (string, error)
}

// FileIcon returns a custom icon from a folder or the default octicon for displaying files/directories
func FileIcon(ctx context.Context, entry *git.TreeEntry) template.HTML {
	backend := &fileIconHTTPBackend{
		theme:   setting.UI.FileIconTheme,
		baseURL: "https://raw.githubusercontent.com/anbraten/gitea-icons/refs/heads/master/gitea/",
	}

	iconTheme := setting.UI.FileIconTheme
	if iconTheme != "" {
		iconNames := getFileIconNames(entry)

		// Try to load the custom icon
		for _, iconName := range iconNames {
			if icon, err := backend.GetIcon(iconName); err == nil {
				if icon, ok := fileIconCache.Get(iconName); ok {
					return svg.RenderHTMLFromString(icon)
				}

				fileIconCache.Add(iconName, string(icon))

				return svg.RenderHTMLFromString(string(icon))
			}
		}
	}

	// If no custom icon was found or an error occurred, return the default icon
	return svg.RenderHTML(getBasicFileIconName(entry))
}
