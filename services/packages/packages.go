// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
)

// CreatePackage creates a new package
func CreatePackage(creator *models.User, repository *models.Repository, packageType models.PackageType, name, version string, metadata interface{}, allowDuplicate bool) (*models.Package, error) {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		log.Error("Error converting metadata to JSON: %v", err)
		return nil, err
	}

	log.Trace("Creating package: %v, %v, %v, %s, %s, %+v, %v", creator.ID, repository.ID, packageType, name, version, metadata, allowDuplicate)

	p := &models.Package{
		RepoID:      repository.ID,
		CreatorID:   creator.ID,
		Type:        packageType,
		Name:        name,
		LowerName:   strings.ToLower(name),
		Version:     version,
		MetadataRaw: string(metadataJSON),
	}
	if p, err = models.TryInsertPackage(p); err != nil {
		if err == models.ErrDuplicatePackage {
			if allowDuplicate {
				return p, nil
			}
			return nil, err
		}
		log.Error("Error inserting package: %v", err)
		return nil, err
	}
	return p, nil
}

// AddFileToPackage adds a new file to package and stores its content
func AddFileToPackage(p *models.Package, filename string, size int64, r io.Reader) (*models.PackageFile, error) {
	log.Trace("Creating package file: %v, %v, %s", p.ID, size, filename)

	pf := &models.PackageFile{
		PackageID: p.ID,
		Size:      size,
		Name:      filename,
		LowerName: strings.ToLower(filename),
	}
	var err error
	if pf, err = models.TryInsertPackageFile(pf); err != nil {
		if err == models.ErrDuplicatePackageFile {
			return nil, err
		}
		log.Error("Error inserting package file: %v", err)
		return nil, err
	}

	md5 := md5.New()
	h1 := sha1.New()
	h256 := sha256.New()
	h512 := sha512.New()

	r = io.TeeReader(r, io.MultiWriter(md5, h1, h256, h512))

	contentStore := packages_module.NewContentStore()

	err = func() error {
		err := contentStore.Save(p.ID, pf.ID, r, size)
		if err != nil {
			log.Error("Error saving package file in content store: %v", err)
			return err
		}

		pf.HashMD5 = fmt.Sprintf("%x", md5.Sum(nil))
		pf.HashSHA1 = fmt.Sprintf("%x", h1.Sum(nil))
		pf.HashSHA256 = fmt.Sprintf("%x", h256.Sum(nil))
		pf.HashSHA512 = fmt.Sprintf("%x", h512.Sum(nil))
		if err = models.UpdatePackageFile(pf); err != nil {
			log.Error("Error updating package file: %v", err)
			return err
		}
		return nil
	}()
	if err != nil {
		_ = contentStore.Delete(p.ID, pf.ID)

		if err := models.DeletePackageFileByID(pf.ID); err != nil {
			log.Error("Error deleting package file: %v", err)
		}
		return nil, err
	}
	return pf, nil
}

// DeletePackageByNameAndVersion deletes a package and all associated files
func DeletePackageByNameAndVersion(repository *models.Repository, packageType models.PackageType, name, version string) error {
	log.Trace("Deleting package: %v, %v, %s, %s", repository.ID, packageType, name, version)

	p, err := models.GetPackageByNameAndVersion(repository.ID, packageType, name, version)
	if err != nil {
		if err == models.ErrPackageNotExist {
			return err
		}
		log.Error("Error getting package: %v", err)
		return err
	}

	return deletePackage(p)
}

// DeletePackageByID deletes a package and all associated files
func DeletePackageByID(repository *models.Repository, packageID int64) error {
	log.Trace("Deleting package: %v, %v", repository.ID, packageID)

	p, err := models.GetPackageByID(packageID)
	if err != nil {
		if err == models.ErrPackageNotExist {
			return err
		}
		log.Error("Error getting package: %v", err)
		return err
	}

	if p.RepoID != repository.ID {
		return models.ErrPackageNotExist
	}

	return deletePackage(p)
}

func deletePackage(p *models.Package) error {
	pfs, err := p.GetFiles()
	if err != nil {
		log.Error("Error getting package files: %v", err)
		return err
	}

	contentStore := packages_module.NewContentStore()
	for _, pf := range pfs {
		if err := contentStore.Delete(p.ID, pf.ID); err != nil {
			log.Error("Error deleting package file [%s]: %v", pf.Name, err)
			return err
		}
	}

	if err := models.DeletePackageByID(p.ID); err != nil {
		log.Error("Error deleting package: %v", err)
		return err
	}

	return nil
}

// GetFileStreamByPackageNameAndVersion returns the content of the specific package file
func GetFileStreamByPackageNameAndVersion(repository *models.Repository, packageType models.PackageType, name, version, filename string) (io.ReadCloser, *models.PackageFile, error) {
	log.Trace("Getting package file stream: %v, %v, %s, %s, %s", repository.ID, packageType, name, version, filename)

	p, err := models.GetPackageByNameAndVersion(repository.ID, packageType, name, version)
	if err != nil {
		if err == models.ErrPackageNotExist {
			return nil, nil, err
		}
		log.Error("Error getting package: %v", err)
		return nil, nil, err
	}

	return getPackageFileStream(p, filename)
}

// GetFileStreamByPackageID returns the content of the specific package file
func GetFileStreamByPackageID(repository *models.Repository, packageID int64, filename string) (io.ReadCloser, *models.PackageFile, error) {
	log.Trace("Getting package file stream: %v, %v, %s", repository.ID, packageID, filename)

	p, err := models.GetPackageByID(packageID)
	if err != nil {
		if err == models.ErrPackageNotExist {
			return nil, nil, err
		}
		log.Error("Error getting package: %v", err)
		return nil, nil, err
	}

	if p.RepoID != repository.ID {
		return nil, nil, models.ErrPackageNotExist
	}

	return getPackageFileStream(p, filename)
}

func getPackageFileStream(p *models.Package, filename string) (io.ReadCloser, *models.PackageFile, error) {
	pf, err := p.GetFileByName(filename)
	if err != nil {
		return nil, nil, err
	}

	s, err := packages_module.NewContentStore().Get(p.ID, pf.ID)
	return s, pf, err
}
