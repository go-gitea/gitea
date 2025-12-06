// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderplugin

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	render_model "code.gitea.io/gitea/models/render"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/renderplugin"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
)

var errManifestNotFound = errors.New("manifest.json not found in plugin archive")

// InstallFromArchive installs or upgrades a plugin from an uploaded ZIP archive.
// If expectedIdentifier is non-empty the archive must contain the matching plugin id.
func InstallFromArchive(ctx context.Context, upload io.Reader, filename, expectedIdentifier string) (*render_model.Plugin, error) {
	tmpFile, cleanupFile, err := setting.AppDataTempDir("render-plugins").CreateTempFileRandom("upload", "*.zip")
	if err != nil {
		return nil, err
	}
	defer cleanupFile()
	if _, err := io.Copy(tmpFile, upload); err != nil {
		return nil, err
	}
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}

	pluginDir, manifest, cleanupDir, err := extractArchive(tmpFile.Name())
	if err != nil {
		return nil, err
	}
	defer cleanupDir()
	if expectedIdentifier != "" && manifest.ID != expectedIdentifier {
		return nil, fmt.Errorf("uploaded plugin id %s does not match %s", manifest.ID, expectedIdentifier)
	}

	entryPath := filepath.Join(pluginDir, filepath.FromSlash(manifest.Entry))
	if ok, _ := util.IsExist(entryPath); !ok {
		return nil, fmt.Errorf("plugin entry %s not found", manifest.Entry)
	}
	if err := replacePluginFiles(manifest.ID, pluginDir); err != nil {
		return nil, err
	}

	plug := &render_model.Plugin{
		Identifier:    manifest.ID,
		Name:          manifest.Name,
		Version:       manifest.Version,
		Description:   manifest.Description,
		Source:        strings.TrimSpace(filename),
		Entry:         manifest.Entry,
		FilePatterns:  manifest.FilePatterns,
		FormatVersion: manifest.SchemaVersion,
	}
	if err := render_model.UpsertPlugin(ctx, plug); err != nil {
		return nil, err
	}
	return plug, nil
}

// Delete removes a plugin from disk and database.
func Delete(ctx context.Context, plug *render_model.Plugin) error {
	if err := deletePluginFiles(plug.Identifier); err != nil {
		return err
	}
	return render_model.DeletePlugin(ctx, plug)
}

// SetEnabled toggles plugin availability after verifying assets exist when enabling.
func SetEnabled(ctx context.Context, plug *render_model.Plugin, enabled bool) error {
	if enabled {
		if err := ensureEntryExists(plug); err != nil {
			return err
		}
	}
	return render_model.SetPluginEnabled(ctx, plug, enabled)
}

// BuildMetadata returns metadata for all enabled plugins.
func BuildMetadata(ctx context.Context) ([]renderplugin.Metadata, error) {
	plugs, err := render_model.ListEnabledPlugins(ctx)
	if err != nil {
		return nil, err
	}
	base := setting.AppSubURL + "/assets/render-plugins/"
	metas := make([]renderplugin.Metadata, 0, len(plugs))
	for _, plug := range plugs {
		if plug.FormatVersion != renderplugin.SupportedManifestVersion {
			log.Warn("Render plugin %s disabled due to incompatible schema version %d", plug.Identifier, plug.FormatVersion)
			continue
		}
		if err := ensureEntryExists(plug); err != nil {
			log.Error("Render plugin %s entry missing: %v", plug.Identifier, err)
			continue
		}
		assetsBase := base + plug.Identifier + "/"
		metas = append(metas, renderplugin.Metadata{
			ID:           plug.Identifier,
			Name:         plug.Name,
			Version:      plug.Version,
			Description:  plug.Description,
			Entry:        plug.Entry,
			EntryURL:     assetsBase + plug.Entry,
			AssetsBase:   assetsBase,
			FilePatterns: append([]string(nil), plug.FilePatterns...),
			SchemaVersion: plug.FormatVersion,
		})
	}
	return metas, nil
}

func ensureEntryExists(plug *render_model.Plugin) error {
	entryPath := renderplugin.ObjectPath(plug.Identifier, filepath.ToSlash(plug.Entry))
	if _, err := renderplugin.Storage().Stat(entryPath); err != nil {
		return fmt.Errorf("plugin entry %s missing: %w", plug.Entry, err)
	}
	return nil
}

func extractArchive(zipPath string) (string, *renderplugin.Manifest, func(), error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", nil, nil, err
	}

	extractDir, cleanup, err := setting.AppDataTempDir("render-plugins").MkdirTempRandom("extract", "*")
	if err != nil {
		_ = reader.Close()
		return "", nil, nil, err
	}

	closeAll := func() {
		_ = reader.Close()
		cleanup()
	}

	for _, file := range reader.File {
		if err := extractZipEntry(file, extractDir); err != nil {
			closeAll()
			return "", nil, nil, err
		}
	}

	manifestPath, err := findManifest(extractDir)
	if err != nil {
		closeAll()
		return "", nil, nil, err
	}
	manifestDir := filepath.Dir(manifestPath)
	manifest, err := renderplugin.LoadManifest(manifestDir)
	if err != nil {
		closeAll()
		return "", nil, nil, err
	}

	return manifestDir, manifest, closeAll, nil
}

func extractZipEntry(file *zip.File, dest string) error {
	cleanRel := util.PathJoinRelX(file.Name)
	if cleanRel == "" || cleanRel == "." {
		return nil
	}
	target := filepath.Join(dest, filepath.FromSlash(cleanRel))
	rel, err := filepath.Rel(dest, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("archive path %q escapes extraction directory", file.Name)
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(target, os.ModePerm)
	}
	if file.FileInfo().Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlinks are not supported inside plugin archives: %s", file.Name)
	}
	if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
		return err
	}
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, rc); err != nil {
		return err
	}
	return nil
}

func findManifest(root string) (string, error) {
	var manifestPath string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "manifest.json") {
			if manifestPath != "" {
				return fmt.Errorf("multiple manifest.json files found")
			}
			manifestPath = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if manifestPath == "" {
		return "", errManifestNotFound
	}
	return manifestPath, nil
}

func replacePluginFiles(identifier, srcDir string) error {
	if err := deletePluginFiles(identifier); err != nil {
		return err
	}
	return uploadPluginDir(identifier, srcDir)
}

func deletePluginFiles(identifier string) error {
	store := renderplugin.Storage()
	prefix := renderplugin.ObjectPrefix(identifier)
	return store.IterateObjects(prefix, func(path string, obj storage.Object) error {
		_ = obj.Close()
		return store.Delete(path)
	})
}

func uploadPluginDir(identifier, src string) error {
	store := renderplugin.Storage()
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported inside plugin archives")
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			return err
		}
		objectPath := renderplugin.ObjectPath(identifier, filepath.ToSlash(rel))
		_, err = store.Save(objectPath, file, info.Size())
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
}
