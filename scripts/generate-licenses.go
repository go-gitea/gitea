// +build ignore

package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	var (
		prefix      = "gitea-licenses"
		url         = "https://api.github.com/repos/spdx/license-list-data/tarball"
		destination = ""
	)

	flag.StringVar(&destination, "dest", "options/license/", "destination for the licenses")
	flag.Parse()

	file, err := ioutil.TempFile(os.TempDir(), prefix)

	if err != nil {
		log.Fatalf("Failed to create temp file. %s", err)
	}

	defer os.Remove(file.Name())

	resp, err := http.Get(url)

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

		if strings.HasPrefix(filepath.Base(hdr.Name), "README") {
			continue
		}

		if strings.HasPrefix(filepath.Base(hdr.Name), "deprecated_") {
			continue
		}
		out, err := os.Create(path.Join(destination, strings.TrimSuffix(filepath.Base(hdr.Name), ".txt")))

		if err != nil {
			log.Fatalf("Failed to create new file. %s", err)
		}

		defer out.Close()

		if _, err := io.Copy(out, tr); err != nil {
			log.Fatalf("Failed to write new file. %s", err)
		} else {
			fmt.Printf("Written %s\n", out.Name())
		}
	}

	fmt.Println("Done")
}
