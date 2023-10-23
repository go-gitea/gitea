//go:build ignore

package main

import (
	"archive/zip"
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"os"

	"code.gitea.io/gitea/modules/util"
)

func main() {
	const (
		url      = "https://timezonedb.com/files/TimeZoneDB.csv.zip"
		dest     = "options/timezones.csv.gz"
		prefix   = "timezone-archive"
		filename = "time_zone.csv"
	)

	file, err := os.CreateTemp(os.TempDir(), prefix)
	if err != nil {
		log.Fatalf("Failed to create temp file. %s", err)
	}

	defer util.Remove(file.Name())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to download archive. %s", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to download archive. %s", err)
	}

	defer resp.Body.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		log.Fatalf("Failed to copy archive to file. %s", err)
	}

	file.Close()

	zf, err := zip.OpenReader(file.Name())
	if err != nil {
		log.Fatalf("Failed to open archive. %s", err)
	}
	defer zf.Close()

	fi, err := zf.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open file in archive. %s", err)
	}
	defer fi.Close()

	fo, err := os.Create(dest)
	if err != nil {
		log.Fatalf("Failed to create file. %s", err)
	}
	defer fo.Close()

	zo := gzip.NewWriter(fo)
	defer zo.Close()

	buf := make([]byte, 1024)
	for {
		// read a chunk
		n, err := fi.Read(buf)
		if err != nil && err != io.EOF {
			log.Fatalf("Failed to read file. %s", err)
		}
		if n == 0 {
			break
		}

		// write a chunk
		if _, err := zo.Write(buf[:n]); err != nil {
			log.Fatalf("Failed to write file. %s", err)
		}
	}
}
