//go:build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	var destination string
	flag.StringVar(&destination, "dest", "options/fileicon/", "destination for the fileicon")
	flag.Parse()

	pkgName := "material-icon-theme"
	req, err := http.NewRequest("GET", fmt.Sprintf("https://registry.npmjs.org/%s/", pkgName), nil)
	if err != nil {
		log.Fatalf("http req: %s", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("http error: %s", err)
	}
	d := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	var m struct {
		DistTags map[string]string `json:"dist-tags"`
	}
	err = d.Decode(&m)
	if err != nil {
		log.Fatalf("json decode: %s", err)
	}

	latestTag := m.DistTags["latest"]
	if latestTag == "" {
		log.Fatal("no latest tag")
	}

	pkg := fmt.Sprintf("https://registry.npmjs.org/%s/-/%s-%s.tgz", pkgName, pkgName, latestTag)
	req, err = http.NewRequest("GET", pkg, nil)
	if err != nil {
		log.Fatalf("http req: %s", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("http error: %s", err)
	}
	defer resp.Body.Close()

	localFileName := filepath.Join(destination, "material.tgz")
	localFile, err := os.Create(localFileName)
	if err != nil {
		log.Fatalf("create file: %s", err)
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, resp.Body)
	if err != nil {
		log.Fatalf("copy body to file: %s", err)
	}

	log.Printf("Downloaded %s to %s", pkg, localFileName)
}
