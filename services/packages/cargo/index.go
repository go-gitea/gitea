// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cargo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"

	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	cargo_module "code.gitea.io/gitea/modules/packages/cargo"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"
	files_service "code.gitea.io/gitea/services/repository/files"
)

const (
	IndexRepositoryName = "_cargo-index"
	ConfigFileName      = "config.json"
)

// https://doc.rust-lang.org/cargo/reference/registries.html#index-format

func BuildPackagePath(name string) string {
	switch len(name) {
	case 0:
		panic("Cargo package name can not be empty")
	case 1:
		return path.Join("1", name)
	case 2:
		return path.Join("2", name)
	case 3:
		return path.Join("3", string(name[0]), name)
	default:
		return path.Join(name[0:2], name[2:4], name)
	}
}

func InitializeIndexRepository(ctx context.Context, doer, owner *user_model.User) error {
	repo, err := getOrCreateIndexRepository(ctx, doer, owner)
	if err != nil {
		return err
	}

	if err := createOrUpdateConfigFile(ctx, repo, doer, owner); err != nil {
		return fmt.Errorf("createOrUpdateConfigFile: %w", err)
	}

	return nil
}

func RebuildIndex(ctx context.Context, doer, owner *user_model.User) error {
	repo, err := getOrCreateIndexRepository(ctx, doer, owner)
	if err != nil {
		return err
	}

	ps, err := packages_model.GetPackagesByType(ctx, owner.ID, packages_model.TypeCargo)
	if err != nil {
		return fmt.Errorf("GetPackagesByType: %w", err)
	}

	return alterRepositoryContent(
		ctx,
		doer,
		repo,
		"Rebuild Cargo Index",
		func(t *files_service.TemporaryUploadRepository) error {
			// Remove all existing content but the Cargo config
			files, err := t.LsFiles(ctx)
			if err != nil {
				return err
			}
			for i, file := range files {
				if file == ConfigFileName {
					files[i] = files[len(files)-1]
					files = files[:len(files)-1]
					break
				}
			}
			if err := t.RemoveFilesFromIndex(ctx, files...); err != nil {
				return err
			}

			// Add all packages
			for _, p := range ps {
				if err := addOrUpdatePackageIndex(ctx, t, p); err != nil {
					return err
				}
			}

			return nil
		},
	)
}

func UpdatePackageIndexIfExists(ctx context.Context, doer, owner *user_model.User, packageID int64) error {
	// We do not want to force the creation of the repo here
	// cargo http index does not rely on the repo itself,
	// so if the repo does not exist, we just do nothing.
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner.Name, IndexRepositoryName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("GetRepositoryByOwnerAndName: %w", err)
	}

	p, err := packages_model.GetPackageByID(ctx, packageID)
	if err != nil {
		return fmt.Errorf("GetPackageByID[%d]: %w", packageID, err)
	}

	return alterRepositoryContent(
		ctx,
		doer,
		repo,
		"Update "+p.Name,
		func(t *files_service.TemporaryUploadRepository) error {
			return addOrUpdatePackageIndex(ctx, t, p)
		},
	)
}

type IndexVersionEntry struct {
	Name         string                     `json:"name"`
	Version      string                     `json:"vers"`
	Dependencies []*cargo_module.Dependency `json:"deps"`
	FileChecksum string                     `json:"cksum"`
	Features     map[string][]string        `json:"features"`
	Yanked       bool                       `json:"yanked"`
	Links        string                     `json:"links,omitempty"`
}

func BuildPackageIndex(ctx context.Context, p *packages_model.Package) (*bytes.Buffer, error) {
	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID: p.ID,
		Sort:      packages_model.SortVersionAsc,
	})
	if err != nil {
		return nil, fmt.Errorf("SearchVersions[%s]: %w", p.Name, err)
	}
	if len(pvs) == 0 {
		return nil, nil
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		return nil, fmt.Errorf("GetPackageDescriptors[%s]: %w", p.Name, err)
	}

	var b bytes.Buffer
	for _, pd := range pds {
		metadata := pd.Metadata.(*cargo_module.Metadata)

		dependencies := metadata.Dependencies
		if dependencies == nil {
			dependencies = make([]*cargo_module.Dependency, 0)
		}

		features := metadata.Features
		if features == nil {
			features = make(map[string][]string)
		}

		yanked, _ := strconv.ParseBool(pd.VersionProperties.GetByName(cargo_module.PropertyYanked))
		entry, err := json.Marshal(&IndexVersionEntry{
			Name:         pd.Package.Name,
			Version:      pd.Version.Version,
			Dependencies: dependencies,
			FileChecksum: pd.Files[0].Blob.HashSHA256,
			Features:     features,
			Yanked:       yanked,
			Links:        metadata.Links,
		})
		if err != nil {
			return nil, err
		}

		b.Write(entry)
		b.WriteString("\n")
	}

	return &b, nil
}

func addOrUpdatePackageIndex(ctx context.Context, t *files_service.TemporaryUploadRepository, p *packages_model.Package) error {
	b, err := BuildPackageIndex(ctx, p)
	if err != nil {
		return err
	}
	if b == nil {
		return nil
	}

	return writeObjectToIndex(ctx, t, BuildPackagePath(p.LowerName), b)
}

func getOrCreateIndexRepository(ctx context.Context, doer, owner *user_model.User) (*repo_model.Repository, error) {
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner.Name, IndexRepositoryName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			repo, err = repo_service.CreateRepositoryDirectly(ctx, doer, owner, repo_service.CreateRepoOptions{
				Name: IndexRepositoryName,
			})
			if err != nil {
				return nil, fmt.Errorf("CreateRepository: %w", err)
			}
		} else {
			return nil, fmt.Errorf("GetRepositoryByOwnerAndName: %w", err)
		}
	}

	return repo, nil
}

type Config struct {
	DownloadURL  string `json:"dl"`
	APIURL       string `json:"api"`
	AuthRequired bool   `json:"auth-required"`
}

func BuildConfig(owner *user_model.User, isPrivate bool) *Config {
	return &Config{
		DownloadURL:  setting.AppURL + "api/packages/" + owner.Name + "/cargo/api/v1/crates",
		APIURL:       setting.AppURL + "api/packages/" + owner.Name + "/cargo",
		AuthRequired: isPrivate,
	}
}

func createOrUpdateConfigFile(ctx context.Context, repo *repo_model.Repository, doer, owner *user_model.User) error {
	return alterRepositoryContent(
		ctx,
		doer,
		repo,
		"Initialize Cargo Config",
		func(t *files_service.TemporaryUploadRepository) error {
			var b bytes.Buffer
			err := json.NewEncoder(&b).Encode(BuildConfig(owner, setting.Service.RequireSignInView || owner.Visibility != structs.VisibleTypePublic || repo.IsPrivate))
			if err != nil {
				return err
			}

			return writeObjectToIndex(ctx, t, ConfigFileName, &b)
		},
	)
}

// This is a shorter version of CreateOrUpdateRepoFile which allows to perform multiple actions on a git repository
func alterRepositoryContent(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commitMessage string, fn func(*files_service.TemporaryUploadRepository) error) error {
	t, err := files_service.NewTemporaryUploadRepository(repo)
	if err != nil {
		return err
	}
	defer t.Close()

	var lastCommitID string
	if err := t.Clone(ctx, repo.DefaultBranch, true); err != nil {
		if !git.IsErrBranchNotExist(err) || !repo.IsEmpty {
			return err
		}
		if err := t.Init(ctx, repo.ObjectFormatName); err != nil {
			return err
		}
	} else {
		if err := t.SetDefaultIndex(ctx); err != nil {
			return err
		}

		commit, err := t.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			return err
		}

		lastCommitID = commit.ID.String()
	}

	if err := fn(t); err != nil {
		return err
	}

	treeHash, err := t.WriteTree(ctx)
	if err != nil {
		return err
	}

	commitOpts := &files_service.CommitTreeUserOptions{
		ParentCommitID: lastCommitID,
		TreeHash:       treeHash,
		CommitMessage:  commitMessage,
		DoerUser:       doer,
	}
	commitHash, err := t.CommitTree(ctx, commitOpts)
	if err != nil {
		return err
	}

	return t.Push(ctx, doer, commitHash, repo.DefaultBranch)
}

func writeObjectToIndex(ctx context.Context, t *files_service.TemporaryUploadRepository, path string, r io.Reader) error {
	hash, err := t.HashObject(ctx, r)
	if err != nil {
		return err
	}

	return t.AddObjectToIndex(ctx, "100644", hash, path)
}
