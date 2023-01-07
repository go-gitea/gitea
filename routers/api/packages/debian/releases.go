package debian

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/clearsign"
	"github.com/keybase/go-crypto/openpgp/packet"
)

func CreateRelease(ctx *context.Context) error {
	basePath := filepath.Join(setting.AppDataPath, "debian_repo", ctx.Package.Owner.Name)
	releasePath := filepath.Join(basePath, "Release")

	releaseFile, err := os.Create(releasePath)
	if err != nil {
		return fmt.Errorf("Failed to create Release file: %s", err)
	}
	defer releaseFile.Close()

	allFiles, err := GetDebianFilesByArch(ctx)
	if err != nil {
		return err
	}

	archs := make([]string, 0)
	for a := range allFiles {
		archs = append(archs, a)
	}

	currentTime := time.Now().UTC()
	fmt.Fprintf(releaseFile, "Suite: gitea\n")
	fmt.Fprintf(releaseFile, "Codename: gitea\n")
	fmt.Fprintf(releaseFile, "Components: main\n")
	fmt.Fprintf(releaseFile, "Architectures: %s\n", strings.Join(archs, " "))
	fmt.Fprintf(releaseFile, "Date: %s\n", currentTime.Format(time.RFC1123))

	var md5sums, sha1sums, sha256sums strings.Builder

	err = filepath.Walk(basePath, func(path string, file os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(path, "Packages.gz") || strings.HasSuffix(path, "Packages") {
			var (
				md5hash    = md5.New()
				sha1hash   = sha1.New()
				sha256hash = sha256.New()
			)

			relPath, _ := filepath.Rel(basePath, path)
			relPath = filepath.Join("main", relPath)
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("Error opening Packages file for reading: %s", err)
			}

			if _, err = io.Copy(io.MultiWriter(md5hash, sha1hash, sha256hash), f); err != nil {
				return fmt.Errorf("Error hashing file for Release list: %s", err)
			}

			fmt.Fprintf(&md5sums, " %s %d %s\n", hex.EncodeToString(md5hash.Sum(nil)), file.Size(), relPath)
			fmt.Fprintf(&sha1sums, " %s %d %s\n", hex.EncodeToString(sha1hash.Sum(nil)), file.Size(), relPath)
			fmt.Fprintf(&sha256sums, " %s %d %s\n", hex.EncodeToString(sha256hash.Sum(nil)), file.Size(), relPath)
			f = nil
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("Error scanning for Package files: %s", err)
	}

	releaseFile.WriteString("MD5Sum:\n")
	releaseFile.WriteString(md5sums.String())
	releaseFile.WriteString("SHA1:\n")
	releaseFile.WriteString(sha1sums.String())
	releaseFile.WriteString("SHA256:\n")
	releaseFile.WriteString(sha256sums.String())

	if err = SignRelease(ctx); err != nil {
		return fmt.Errorf("Error signing Release file: %s", err)
	}

	return nil
}

func GetEntity() (*openpgp.Entity, error) {
	keyPath := filepath.Join(setting.AppDataPath, "debian.gpg")
	keyFile, err := os.Open(keyPath)
	if err != nil {
		return nil, fmt.Errorf("Error opening key file: %s", err)
	}
	defer keyFile.Close()

	r := packet.NewReader(keyFile)
	return openpgp.ReadEntity(r)
}

func SignRelease(ctx *context.Context) error {
	e, err := GetEntity()
	if err != nil {
		return fmt.Errorf("Cannot read entity from key file: %s", err)
	}

	basePath := filepath.Join(setting.AppDataPath, "debian_repo", ctx.Package.Owner.Name)
	releasePath := filepath.Join(basePath, "Release")
	releaseGzPath := filepath.Join(basePath, "Release.gpg")
	inreleasePath := filepath.Join(basePath, "InRelease")

	releaseFile, err := os.Open(releasePath)
	if err != nil {
		return fmt.Errorf("Error opening Release file (%s) for reading: %s", releasePath, err)
	}
	defer releaseFile.Close()

	releaseGzFile, err := os.Create(releaseGzPath)
	if err != nil {
		return fmt.Errorf("Error opening Release.gpg file (%s) for writing: %s", releaseGzPath, err)
	}
	defer releaseGzFile.Close()

	err = openpgp.ArmoredDetachSign(releaseGzFile, e, releaseFile, nil)
	if err != nil {
		return fmt.Errorf("Error writing signature to Release.gpg file: %s", err)
	}

	releaseFile.Seek(0, 0)

	inreleaseFile, err := os.Create(inreleasePath)
	if err != nil {
		return fmt.Errorf("Error opening InRelease file (%s) for writing: %s", inreleasePath, err)
	}
	defer inreleaseFile.Close()

	writer, err := clearsign.Encode(inreleaseFile, e.PrivateKey, nil)
	if err != nil {
		return fmt.Errorf("Error signing InRelease file: %s", err)
	}

	io.Copy(writer, releaseFile)
	writer.Close()

	return nil
}
