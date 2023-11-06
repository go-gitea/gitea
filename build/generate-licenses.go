//go:build ignore

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

func main() {
	var (
		prefix                  = "gitea-licenses"
		url                     = "https://api.github.com/repos/spdx/license-list-data/tarball"
		githubApiToken          = ""
		githubUsername          = ""
		destination             = ""
		sameLicensesDestination = ""
	)

	flag.StringVar(&destination, "dest", "options/license/", "destination for the licenses")
	flag.StringVar(&sameLicensesDestination, "samelicensesdest", "options/sameLicenses", "destination for same license json")
	flag.StringVar(&githubUsername, "username", "", "github username")
	flag.StringVar(&githubApiToken, "token", "", "github api token")
	flag.Parse()

	file, err := os.CreateTemp(os.TempDir(), prefix)
	if err != nil {
		log.Fatalf("Failed to create temp file. %s", err)
	}

	defer util.Remove(file.Name())

	if err := os.RemoveAll(destination); err != nil {
		log.Fatalf("Cannot clean destination folder: %v", err)
	}

	if err := os.MkdirAll(destination, 0o755); err != nil {
		log.Fatalf("Cannot create destination: %v", err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to download archive. %s", err)
	}

	if len(githubApiToken) > 0 && len(githubUsername) > 0 {
		req.SetBasicAuth(githubUsername, githubApiToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to download archive. %s", err)
	}

	defer resp.Body.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		log.Fatalf("Failed to copy archive to file. %s", err)
	}

	if _, err := file.Seek(0, 0); err != nil {
		log.Fatalf("Failed to reset seek on archive. %s", err)
	}

	gz, err := gzip.NewReader(file)
	if err != nil {
		log.Fatalf("Failed to gunzip the archive. %s", err)
	}

	tr := tar.NewReader(gz)
	var preFile *os.File
	var preLicenseName string
	sameFiles := make(map[string][]string)
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("Failed to iterate archive. %s", err)
		}

		if !strings.Contains(hdr.Name, "/text/") {
			continue
		}

		if filepath.Ext(hdr.Name) != ".txt" {
			continue
		}

		fileBaseName := filepath.Base(hdr.Name)
		licenseName := strings.TrimSuffix(fileBaseName, ".txt")

		if strings.HasPrefix(fileBaseName, "README") {
			continue
		}

		if strings.HasPrefix(fileBaseName, "deprecated_") {
			continue
		}
		out, err := os.Create(path.Join(destination, licenseName))
		if err != nil {
			log.Fatalf("Failed to create new file. %s", err)
		}

		defer out.Close()

		if _, err := io.Copy(out, tr); err != nil {
			log.Fatalf("Failed to write new file. %s", err)
		} else {
			fmt.Printf("Written %s\n", out.Name())

			// some license files have same content, so we need to detect these files and create a convert map into a file
			// In InitClassifier, we will use this convert map to avoid adding same license content with different license name
			md5, err := getSameFileMD5(preFile, out)
			if err != nil {
				log.Fatalf("Failed to get same file md5. %s", err)
				continue
			}
			if md5 != "" {
				_, ok := sameFiles[md5]
				if !ok {
					sameFiles[md5] = make([]string, 0)
				}
				if !slices.Contains(sameFiles[md5], preLicenseName) {
					sameFiles[md5] = append(sameFiles[md5], preLicenseName)
				}
				sameFiles[md5] = append(sameFiles[md5], licenseName)
			}
			preFile = out
			preLicenseName = licenseName
		}
	}

	// generate convert license name map
	sameLicenses := make(map[string]string)
	for _, fileNames := range sameFiles {
		key := getLicenseKey(fileNames)
		for _, fileName := range fileNames {
			sameLicenses[fileName] = key
		}
	}
	// save convert license name map to file
	bytes, err := json.Marshal(sameLicenses)
	if err != nil {
		log.Fatalf("Failed to create json bytes. %s", err)
		return
	}
	out, err := os.Create(sameLicensesDestination)
	if err != nil {
		log.Fatalf("Failed to create new file. %s", err)
	}
	defer out.Close()
	_, err = out.Write(bytes)
	if err != nil {
		log.Fatalf("Failed to write same licenses json file. %s", err)
	}

	fmt.Println("Done")
}

// getSameFileMD5 returns md5 of the input file, if the content of input files are same
func getSameFileMD5(f1, f2 *os.File) (string, error) {
	if f1 == nil || f2 == nil {
		return "", nil
	}

	// check file size
	fs1, err := f1.Stat()
	if err != nil {
		return "", err
	}
	fs2, err := f2.Stat()
	if err != nil {
		return "", err
	}

	if fs1.Size() != fs2.Size() {
		return "", nil
	}

	// check content
	chunkSize := 1024
	_, err = f1.Seek(0, 0)
	if err != nil {
		return "", err
	}
	_, err = f2.Seek(0, 0)
	if err != nil {
		return "", err
	}

	var totalBytes []byte
	for {
		b1 := make([]byte, chunkSize)
		_, err1 := f1.Read(b1)

		b2 := make([]byte, chunkSize)
		_, err2 := f2.Read(b2)

		totalBytes = append(totalBytes, b1...)

		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				md5 := md5.Sum(totalBytes)
				return string(md5[:]), nil
			} else if err1 == io.EOF || err2 == io.EOF {
				return "", nil
			} else if err1 != nil {
				return "", err1
			} else if err2 != nil {
				return "", err2
			}
		}

		if !bytes.Equal(b1, b2) {
			return "", nil
		}
	}
}

func getLicenseKey(fnl []string) string {
	if len(fnl) == 0 {
		return ""
	}

	shortestItem := func(list []string) string {
		s := list[0]
		for _, l := range list[1:] {
			if len(l) < len(s) {
				s = l
			}
		}
		return s
	}
	allHasPrefix := func(list []string, s string) bool {
		for _, l := range list {
			if !strings.HasPrefix(l, s) {
				return false
			}
		}
		return true
	}

	sl := shortestItem(fnl)
	slv := strings.Split(sl, "-")
	var result string
	for i := len(slv); i >= 0; i-- {
		result = strings.Join(slv[:i], "-")
		if allHasPrefix(fnl, result) {
			return result
		}
	}
	return ""
}
