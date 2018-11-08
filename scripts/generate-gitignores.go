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
		prefix      = "gitea-gitignore"
		url         = "https://api.github.com/repos/github/gitignore/tarball"
		destination = ""
	)

	flag.StringVar(&destination, "dest", "options/gitignore/", "destination for the gitignores")
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

	filesToCopy := make(map[string]string, 0)

	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("Failed to iterate archive. %s", err)
		}

		if filepath.Ext(hdr.Name) != ".gitignore" {
			continue
		}

		if hdr.Typeflag == tar.TypeSymlink {
			fmt.Printf("Found symlink %s -> %s\n", hdr.Name, hdr.Linkname)
			filesToCopy[strings.TrimSuffix(filepath.Base(hdr.Name), ".gitignore")] = strings.TrimSuffix(filepath.Base(hdr.Linkname), ".gitignore")
			continue
		}

		out, err := os.Create(path.Join(destination, strings.TrimSuffix(filepath.Base(hdr.Name), ".gitignore")))

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

	for dst, src := range filesToCopy {
		// Read all content of src to data
		src = path.Join(destination, src)
		data, err := ioutil.ReadFile(src)
		if err != nil {
			log.Fatalf("Failed to read src file. %s", err)
		}
		// Write data to dst
		dst = path.Join(destination, dst)
		err = ioutil.WriteFile(dst, data, 0644)
		if err != nil {
			log.Fatalf("Failed to write new file. %s", err)
		}
		fmt.Printf("Written (copy of %s) %s\n", src, dst)
	}

	fmt.Println("Done")
}
