package debian

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/packages"
	"github.com/blakesmith/ar"
	lzma "github.com/xi2/xz"
)

type Compression int

const (
	LZMA Compression = iota
	GZIP
)

func CreatePackagesGz(ctx *context.Context, arch string) error {
	basePath := filepath.Join(setting.AppDataPath, "debian_repo")

	archDir := arch
	if arch != "source" {
		archDir = "binary-" + archDir
	}
	archPath := filepath.Join(basePath, ctx.Package.Owner.Name, archDir)

	info, err := os.Stat(archPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err2 := os.MkdirAll(archPath, 0775); err2 != nil {
				return fmt.Errorf("Cannot create Debian repo path: %s", err)
			}
		} else {
			return fmt.Errorf("Cannot stat Debian repo path: %s", err)
		}
	} else {
		if !info.IsDir() {
			return fmt.Errorf("Debian repo path is not a directory!")
		}
	}

	packagesPath := filepath.Join(archPath, "Packages")
	packagesGzPath := filepath.Join(archPath, "Packages.gz")

	packagesFile, err := os.Create(packagesPath)
	if err != nil {
		return fmt.Errorf("Failed to create Packages file %s: %s", packagesPath, err)
	}
	defer packagesFile.Close()

	packagesGzFile, err := os.Create(packagesGzPath)
	if err != nil {
		return fmt.Errorf("Failed to create Packages.gz file %s: %s", packagesGzPath, err)
	}
	defer packagesGzFile.Close()

	gzOut := gzip.NewWriter(packagesGzFile)
	defer gzOut.Close()

	writer := io.MultiWriter(packagesFile, gzOut)

	allFiles, err := GetDebianFilesByArch(ctx)
	if err != nil {
		return err
	}

	archFiles, exists := allFiles[arch]
	if !exists {
		return fmt.Errorf("No files in arch \"%s\"!", arch)
	}

	for i, file := range archFiles {
		var packBuf bytes.Buffer
		tempCtlData, err := InspectPackage(ctx, file)
		if err != nil {
			return err
		}

		blob, err := packages_model.GetBlobByID(ctx, file.File.BlobID)
		if err != nil {
			return err
		}

		packBuf.WriteString(tempCtlData)
		dir := filepath.Join("/pool", file.File.Name)
		fmt.Fprintf(&packBuf, "Filename: %s\n", dir)
		fmt.Fprintf(&packBuf, "Size: %d\n", blob.Size)
		fmt.Fprintf(&packBuf, "MD5sum: %s\n", blob.HashMD5)
		fmt.Fprintf(&packBuf, "SHA1: %s\n", blob.HashSHA1)
		fmt.Fprintf(&packBuf, "SHA256: %s\n", blob.HashSHA256)

		if i != (len(archFiles) - 1) {
			packBuf.WriteString("\n\n")
		}
		writer.Write(packBuf.Bytes())
	}

	return nil
}

func InspectPackage(ctx *context.Context, fd *packages_model.PackageFileDescriptor) (string, error) {
	// blob, err := packages_model.GetBlobByID(ctx, file.File.BlobID)
	// if err != nil {
	// 	return "", err
	// }

	fstream, _, err := packages.GetFileStreamByPackageVersionAndFileID(ctx, ctx.Package.Owner, fd.File.VersionID, fd.File.ID)
	if err != nil {
		return "", err
	}
	defer fstream.Close()
	reader := ar.NewReader(fstream)

	var controlBuf bytes.Buffer
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return "", fmt.Errorf("Error in inspect loop: %s", err)
		}

		if strings.Contains(header.Name, "control.tar") {
			var compression Compression
			switch strings.TrimRight(header.Name, "/") {
			case "control.tar.gz":
				compression = GZIP
			case "control.tar.xz":
				compression = LZMA
			default:
				return "", errors.New("No control file found")
			}

			io.Copy(&controlBuf, reader)
			return InspectPackageControl(compression, controlBuf)
		}
	}

	return "", errors.New("Unreachable?")
}

func InspectPackageControl(comp Compression, data bytes.Buffer) (string, error) {
	var tarReader *tar.Reader
	var err error

	switch comp {
	case GZIP:
		var compFile *gzip.Reader
		compFile, err = gzip.NewReader(bytes.NewReader(data.Bytes()))
		tarReader = tar.NewReader(compFile)
	case LZMA:
		var compFile *lzma.Reader
		compFile, err = lzma.NewReader(bytes.NewReader(data.Bytes()), lzma.DefaultDictMax)
		tarReader = tar.NewReader(compFile)
	}

	if err != nil {
		return "", fmt.Errorf("Error creating gzip/lzma reader: %s", err)
	}

	var controlBuf bytes.Buffer
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return "", fmt.Errorf("Failed to inspect package: %s", err)
		}

		name := header.Name
		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			switch name {
			case "control", "./control":
				io.Copy(&controlBuf, tarReader)
				return strings.TrimRight(controlBuf.String(), "\n") + "\n", nil
			}
		}
	}

	return "", nil
}
