package fileicon

import (
	"fmt"
	"io"
	"net/http"
	"path"

	"code.gitea.io/gitea/modules/log"
)

type fileIconHTTPBackend struct {
	theme   string
	baseURL string
}

func (f *fileIconHTTPBackend) GetIcon(iconName string) (string, error) {
	iconPath := path.Join(f.baseURL, f.theme, iconName+".svg")

	log.Info("Loading custom icon from %s", iconPath)

	// Try to load the icon via HTTP get
	res, err := http.Get(iconPath)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Failed to load icon: %s", res.Status)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(resBody), nil
}
